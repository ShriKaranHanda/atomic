package cli

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/ShriKaranHanda/atomic/internal/exitcode"
	"github.com/ShriKaranHanda/atomic/internal/ipc"
	"github.com/ShriKaranHanda/atomic/internal/preflight"
)

const defaultSocketPath = "/run/atomicd.sock"

type Config struct {
	SocketPath    string
	KeepArtifacts bool
	Verbose       bool
}

func Run(args []string) int {
	cfg, rest, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitcode.Unsupported
	}
	if err := preflight.CheckClient(); err != nil {
		fmt.Fprintln(os.Stderr, "preflight failed:", err)
		return exitcode.Unsupported
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: atomic [flags] <script_path> [script_args...] | atomic recover")
		return exitcode.Unsupported
	}

	conn, err := net.Dial("unix", cfg.SocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "atomicd is not reachable at %s: %v\n", cfg.SocketPath, err)
		fmt.Fprintln(os.Stderr, "hint: verify atomicd.socket is enabled and running")
		return exitcode.Unsupported
	}
	defer conn.Close()

	writer := ipc.NewWriter(conn)
	reader := ipc.NewReader(conn)

	var req ipc.Request
	if rest[0] == "recover" {
		req = ipc.Request{Type: ipc.RequestRecover, Version: ipc.Version}
	} else {
		scriptPath, scriptArgs, cwd, err := resolveScript(rest)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitcode.Unsupported
		}
		req = ipc.Request{
			Type:          ipc.RequestRun,
			Version:       ipc.Version,
			ScriptPath:    scriptPath,
			ScriptArgs:    scriptArgs,
			CWD:           cwd,
			KeepArtifacts: cfg.KeepArtifacts,
			Verbose:       cfg.Verbose,
		}
	}
	if err := writer.WriteRequest(req); err != nil {
		fmt.Fprintln(os.Stderr, "failed to send request to atomicd:", err)
		return exitcode.Unsupported
	}

	for {
		ev, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(os.Stderr, "atomicd disconnected before returning a result")
				return exitcode.RecoveryFailure
			}
			fmt.Fprintln(os.Stderr, "failed to read response from atomicd:", err)
			return exitcode.RecoveryFailure
		}
		switch ev.Type {
		case ipc.EventStdout:
			data, err := ipc.DecodeData(ev.DataB64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "invalid stdout frame from atomicd:", err)
				return exitcode.RecoveryFailure
			}
			_, _ = os.Stdout.Write(data)
		case ipc.EventStderr:
			data, err := ipc.DecodeData(ev.DataB64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "invalid stderr frame from atomicd:", err)
				return exitcode.RecoveryFailure
			}
			_, _ = os.Stderr.Write(data)
		case ipc.EventError:
			if ev.Message != "" {
				fmt.Fprintln(os.Stderr, ev.Message)
			}
			if ev.AtomicExitCode != 0 {
				return ev.AtomicExitCode
			}
			return exitcode.Unsupported
		case ipc.EventResult:
			if ev.Message != "" && ev.AtomicExitCode != 0 {
				fmt.Fprintln(os.Stderr, ev.Message)
			}
			return ev.AtomicExitCode
		case ipc.EventStart:
			continue
		default:
			continue
		}
	}
}

func parseFlags(args []string) (Config, []string, error) {
	cfg := Config{}
	fs := flag.NewFlagSet("atomic", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.SocketPath, "socket", socketPathFromEnv(), "atomicd unix socket path")
	fs.BoolVar(&cfg.KeepArtifacts, "keep-artifacts", false, "keep run artifacts after completion")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return Config{}, nil, err
	}
	return cfg, fs.Args(), nil
}

func resolveScript(args []string) (string, []string, string, error) {
	scriptPath, err := filepath.Abs(args[0])
	if err != nil {
		return "", nil, "", fmt.Errorf("resolve script path: %w", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return "", nil, "", fmt.Errorf("script path error: %w", err)
	}
	cwd := filepath.Dir(scriptPath)
	return filepath.Clean(scriptPath), args[1:], filepath.Clean(cwd), nil
}

func socketPathFromEnv() string {
	if path := os.Getenv("ATOMIC_SOCKET"); path != "" {
		return path
	}
	return defaultSocketPath
}
