package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/engine"
	"github.com/ShriKaranHanda/atomic/internal/exitcode"
	"github.com/ShriKaranHanda/atomic/internal/ipc"
	"github.com/ShriKaranHanda/atomic/internal/preflight"
)

const DefaultSocketPath = "/run/atomicd.sock"

type Config struct {
	SocketPath string
	StateDir   string
	WorkDir    string
	JournalDir string
	RootPrefix string
}

type Server struct {
	cfg Config

	runMu   sync.Mutex
	running bool
}

func Run(ctx context.Context, cfg Config) error {
	if err := preflight.CheckDaemon(); err != nil {
		return err
	}
	applyDefaults(&cfg)

	recovery := engine.RecoverOnly(cfg.JournalDir, cfg.RootPrefix)
	if recovery.AtomicExitCode != exitcode.OK {
		return fmt.Errorf("startup recovery failed: %s", recovery.Message)
	}

	ln, cleanup, err := listen(cfg.SocketPath)
	if err != nil {
		return err
	}
	defer cleanup()

	srv := &Server{cfg: cfg}
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		errCh <- srv.serve(ctx, ln)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err == nil || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) serve(ctx context.Context, ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := ipc.NewReader(conn)
	writer := ipc.NewWriter(conn)

	req, err := reader.ReadRequest()
	if err != nil {
		if err != io.EOF {
			_ = writer.WriteEvent(ipc.Event{Type: ipc.EventError, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("failed to read request: %v", err)})
		}
		return
	}
	if req.Version != ipc.Version {
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventError, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("unsupported protocol version %d", req.Version)})
		return
	}

	runAsUID, runAsGID, err := peerCredentials(conn)
	if err != nil {
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventError, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("failed to identify caller: %v", err)})
		return
	}

	switch req.Type {
	case ipc.RequestRecover:
		result := engine.RecoverOnly(s.cfg.JournalDir, s.cfg.RootPrefix)
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventResult, AtomicExitCode: result.AtomicExitCode, Message: result.Message})
		return
	case ipc.RequestRun:
		if !s.acquireRun() {
			_ = writer.WriteEvent(ipc.Event{Type: ipc.EventError, AtomicExitCode: exitcode.Unsupported, Message: "atomicd is busy with another transaction"})
			return
		}
		defer s.releaseRun()
		runID := fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), os.Getpid())
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventStart, RunID: runID})

		stdoutWriter := &ipc.StreamEventWriter{Kind: ipc.EventStdout, RunID: runID, Sink: writer.WriteEvent}
		stderrWriter := &ipc.StreamEventWriter{Kind: ipc.EventStderr, RunID: runID, Sink: writer.WriteEvent}

		result := engine.Execute(ctx, engine.ExecuteRequest{
			RunID:         runID,
			StateDir:      s.cfg.StateDir,
			WorkDir:       s.cfg.WorkDir,
			JournalDir:    s.cfg.JournalDir,
			RootPrefix:    s.cfg.RootPrefix,
			ScriptPath:    req.ScriptPath,
			ScriptArgs:    req.ScriptArgs,
			CWD:           req.CWD,
			RunAsUID:      runAsUID,
			RunAsGID:      runAsGID,
			KeepArtifacts: req.KeepArtifacts,
			Verbose:       req.Verbose,
			Stdout:        stdoutWriter,
			Stderr:        stderrWriter,
			Stdin:         nil,
		})
		if result.RunID == "" {
			result.RunID = runID
		}
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventResult, RunID: result.RunID, AtomicExitCode: result.AtomicExitCode, ScriptExitCode: result.ScriptExitCode, Message: result.Message})
		return
	default:
		_ = writer.WriteEvent(ipc.Event{Type: ipc.EventError, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("unsupported request type %q", req.Type)})
		return
	}
}

func (s *Server) acquireRun() bool {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Server) releaseRun() {
	s.runMu.Lock()
	s.running = false
	s.runMu.Unlock()
}

func applyDefaults(cfg *Config) {
	if cfg.SocketPath == "" {
		cfg.SocketPath = DefaultSocketPath
	}
	if cfg.StateDir == "" {
		cfg.StateDir = engine.DefaultStateDir
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = engine.DefaultWorkDir
	}
	if cfg.JournalDir == "" {
		cfg.JournalDir = engine.DefaultJournalDir
	}
}

func listen(socketPath string) (net.Listener, func(), error) {
	if ln, ok, err := systemdListener(); err != nil {
		return nil, nil, err
	} else if ok {
		return ln, func() { _ = ln.Close() }, nil
	}

	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, nil, err
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, err
	}
	_ = os.Chmod(socketPath, 0o660)
	cleanup := func() {
		_ = ln.Close()
		_ = os.Remove(socketPath)
	}
	return ln, cleanup, nil
}

func systemdListener() (net.Listener, bool, error) {
	listenFds := strings.TrimSpace(os.Getenv("LISTEN_FDS"))
	listenPID := strings.TrimSpace(os.Getenv("LISTEN_PID"))
	if listenFds == "" || listenPID == "" {
		return nil, false, nil
	}
	fds, err := strconv.Atoi(listenFds)
	if err != nil || fds < 1 {
		return nil, false, nil
	}
	pid, err := strconv.Atoi(listenPID)
	if err != nil || pid != os.Getpid() {
		return nil, false, nil
	}
	f := os.NewFile(uintptr(3), "systemd-activation")
	if f == nil {
		return nil, false, fmt.Errorf("failed to read systemd socket fd")
	}
	ln, err := net.FileListener(f)
	if err != nil {
		f.Close()
		return nil, false, err
	}
	return ln, true, nil
}
