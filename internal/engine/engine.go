package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/commit"
	"github.com/ShriKaranHanda/atomic/internal/conflict"
	"github.com/ShriKaranHanda/atomic/internal/diff"
	"github.com/ShriKaranHanda/atomic/internal/exitcode"
	"github.com/ShriKaranHanda/atomic/internal/journal"
	"github.com/ShriKaranHanda/atomic/internal/overlay"
	"github.com/ShriKaranHanda/atomic/internal/preflight"
	"github.com/ShriKaranHanda/atomic/internal/recover"
)

const (
	DefaultStateDir   = "/var/lib/atomic"
	DefaultWorkDir    = "/var/lib/atomic/runs"
	DefaultJournalDir = "/var/lib/atomic/journal"
)

type ExecuteRequest struct {
	RunID      string
	StateDir   string
	WorkDir    string
	JournalDir string
	RootPrefix string

	ScriptPath string
	ScriptArgs []string
	CWD        string

	RunAsUID uint32
	RunAsGID uint32

	KeepArtifacts bool
	Verbose       bool

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

type ExecuteResult struct {
	RunID          string
	AtomicExitCode int
	ScriptExitCode int
	Message        string
}

func RecoverOnly(journalDir string, rootPrefix string) ExecuteResult {
	if journalDir == "" {
		journalDir = DefaultJournalDir
	}
	if err := preflight.CheckDaemon(); err != nil {
		return ExecuteResult{AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("preflight failed: %v", err)}
	}
	if err := recover.Run(journalDir, rootPrefix); err != nil {
		return ExecuteResult{AtomicExitCode: exitcode.RecoveryFailure, Message: fmt.Sprintf("recovery failed: %v", err)}
	}
	return ExecuteResult{AtomicExitCode: exitcode.OK}
}

func Execute(ctx context.Context, req ExecuteRequest) ExecuteResult {
	applyDefaults(&req)
	if err := preflight.CheckDaemon(); err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("preflight failed: %v", err)}
	}
	if err := recover.Run(req.JournalDir, req.RootPrefix); err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.RecoveryFailure, Message: fmt.Sprintf("recovery failed: %v", err)}
	}
	if req.ScriptPath == "" {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Unsupported, Message: "script path is required"}
	}
	if req.CWD == "" {
		req.CWD = filepath.Dir(req.ScriptPath)
	}

	txnStart := time.Now().UTC()
	res, err := overlay.RunScript(ctx, overlay.RunConfig{
		RunID:      req.RunID,
		WorkRoot:   req.WorkDir,
		ScriptPath: req.ScriptPath,
		ScriptArgs: req.ScriptArgs,
		CWD:        req.CWD,
		RunAsUID:   req.RunAsUID,
		RunAsGID:   req.RunAsGID,
		Verbose:    req.Verbose,
		Stdout:     req.Stdout,
		Stderr:     req.Stderr,
		Stdin:      req.Stdin,
	})
	if err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("overlay run failed: %v", err)}
	}
	if res.ExitCode != 0 {
		if !req.KeepArtifacts {
			_ = os.RemoveAll(res.RunDir)
		}
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.ScriptFailed, ScriptExitCode: res.ExitCode, Message: "script failed"}
	}

	ops := make([]journal.Operation, 0)
	for _, mount := range res.UpperDirs {
		scanned, err := diff.ScanUpperDir(mount.UpperDir, mount.MountPoint)
		if err != nil {
			return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("scan diff failed: %v", err)}
		}
		ops = append(ops, scanned...)
	}
	ops = diff.Plan(ops)
	ops, err = conflict.AttachBaselines(ops)
	if err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Unsupported, Message: fmt.Sprintf("baseline collection failed: %v", err)}
	}
	if err := conflict.Check(ops, txnStart, nil); err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.Conflict, Message: fmt.Sprintf("conflict detected: %v", err)}
	}

	backupDir := filepath.Join(req.StateDir, "backups", req.RunID)
	j := &journal.Journal{
		RunID:         req.RunID,
		State:         journal.StateCommitting,
		Ops:           ops,
		AppliedIndex:  -1,
		BackupRefs:    map[string]journal.BackupRef{},
		StartedAt:     txnStart,
		TxnStart:      txnStart,
		RunDir:        res.RunDir,
		BackupDir:     backupDir,
		KeepArtifacts: req.KeepArtifacts,
	}
	journalPath := filepath.Join(req.JournalDir, req.RunID+".json")
	eng := commit.Engine{RootPrefix: req.RootPrefix}
	if err := eng.Apply(journalPath, j); err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.RecoveryFailure, Message: fmt.Sprintf("commit failed: %v", err)}
	}
	if err := finalize(journalPath, j); err != nil {
		return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.RecoveryFailure, Message: fmt.Sprintf("cleanup failed: %v", err)}
	}
	return ExecuteResult{RunID: req.RunID, AtomicExitCode: exitcode.OK}
}

func applyDefaults(req *ExecuteRequest) {
	if req.StateDir == "" {
		req.StateDir = DefaultStateDir
	}
	if req.WorkDir == "" {
		req.WorkDir = DefaultWorkDir
	}
	if req.JournalDir == "" {
		req.JournalDir = DefaultJournalDir
	}
	if req.RunID == "" {
		now := time.Now().UTC()
		req.RunID = fmt.Sprintf("%d-%d", now.UnixNano(), os.Getpid())
	}
	if req.Stdout == nil {
		req.Stdout = io.Discard
	}
	if req.Stderr == nil {
		req.Stderr = io.Discard
	}
}

func finalize(journalPath string, j *journal.Journal) error {
	if !j.KeepArtifacts {
		if err := os.RemoveAll(j.RunDir); err != nil {
			return err
		}
		if err := os.RemoveAll(j.BackupDir); err != nil {
			return err
		}
	}
	if err := os.Remove(journalPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
