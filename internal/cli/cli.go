package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/commit"
	"github.com/ShriKaranHanda/atomic/internal/conflict"
	"github.com/ShriKaranHanda/atomic/internal/diff"
	"github.com/ShriKaranHanda/atomic/internal/journal"
	"github.com/ShriKaranHanda/atomic/internal/overlay"
	"github.com/ShriKaranHanda/atomic/internal/preflight"
	"github.com/ShriKaranHanda/atomic/internal/recover"
)

const (
	ExitOK              = 0
	ExitScriptFailed    = 10
	ExitUnsupported     = 20
	ExitConflict        = 21
	ExitRecoveryFailure = 30
)

type Config struct {
	StateDir      string
	WorkDir       string
	JournalDir    string
	KeepArtifacts bool
	Verbose       bool
	RootPrefix    string
}

func Run(args []string) int {
	cfg, rest, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ExitUnsupported
	}
	if err := recover.Run(cfg.JournalDir, cfg.RootPrefix); err != nil {
		fmt.Fprintln(os.Stderr, "recovery failed:", err)
		return ExitRecoveryFailure
	}
	if len(rest) > 0 && rest[0] == "recover" {
		return ExitOK
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "usage: atomic [flags] <script_path> [script_args...]")
		return ExitUnsupported
	}
	if err := preflight.Check(); err != nil {
		fmt.Fprintln(os.Stderr, "preflight failed:", err)
		return ExitUnsupported
	}

	scriptPath, scriptArgs, cwd, err := resolveScript(rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ExitUnsupported
	}

	txnStart := time.Now().UTC()
	runID := fmt.Sprintf("%d-%d", txnStart.UnixNano(), os.Getpid())
	res, err := overlay.RunScript(context.Background(), overlay.RunConfig{
		RunID:      runID,
		WorkRoot:   cfg.WorkDir,
		ScriptPath: scriptPath,
		ScriptArgs: scriptArgs,
		CWD:        cwd,
		Verbose:    cfg.Verbose,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "overlay run failed:", err)
		return ExitUnsupported
	}
	if res.ExitCode != 0 {
		if !cfg.KeepArtifacts {
			_ = os.RemoveAll(res.RunDir)
		}
		return ExitScriptFailed
	}

	ops := make([]journal.Operation, 0)
	for _, mount := range res.UpperDirs {
		scanned, err := diff.ScanUpperDir(mount.UpperDir, mount.MountPoint)
		if err != nil {
			fmt.Fprintln(os.Stderr, "scan diff failed:", err)
			return ExitUnsupported
		}
		ops = append(ops, scanned...)
	}
	ops = diff.Plan(ops)
	ops, err = conflict.AttachBaselines(ops)
	if err != nil {
		fmt.Fprintln(os.Stderr, "baseline collection failed:", err)
		return ExitUnsupported
	}
	if err := conflict.Check(ops, txnStart, nil); err != nil {
		fmt.Fprintln(os.Stderr, "conflict detected:", err)
		return ExitConflict
	}

	backupDir := filepath.Join(cfg.StateDir, "backups", runID)
	j := &journal.Journal{
		RunID:         runID,
		State:         journal.StateCommitting,
		Ops:           ops,
		AppliedIndex:  -1,
		BackupRefs:    map[string]journal.BackupRef{},
		StartedAt:     txnStart,
		TxnStart:      txnStart,
		RunDir:        res.RunDir,
		BackupDir:     backupDir,
		KeepArtifacts: cfg.KeepArtifacts,
	}
	journalPath := filepath.Join(cfg.JournalDir, runID+".json")
	eng := commit.Engine{RootPrefix: cfg.RootPrefix}
	if err := eng.Apply(journalPath, j); err != nil {
		fmt.Fprintln(os.Stderr, "commit failed:", err)
		return ExitRecoveryFailure
	}
	if err := finalize(journalPath, j); err != nil {
		fmt.Fprintln(os.Stderr, "cleanup failed:", err)
		return ExitRecoveryFailure
	}
	return ExitOK
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
	if err := os.Remove(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func parseFlags(args []string) (Config, []string, error) {
	cfg := Config{}
	fs := flag.NewFlagSet("atomic", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.StateDir, "state-dir", "/var/lib/atomic", "state directory")
	fs.StringVar(&cfg.WorkDir, "work-dir", "/var/lib/atomic/runs", "run workspace")
	fs.StringVar(&cfg.JournalDir, "journal-dir", "/var/lib/atomic/journal", "journal directory")
	fs.BoolVar(&cfg.KeepArtifacts, "keep-artifacts", false, "keep run artifacts after completion")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "verbose output")
	fs.StringVar(&cfg.RootPrefix, "root-prefix", "", "test-only root prefix")
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
