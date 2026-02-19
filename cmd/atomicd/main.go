package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ShriKaranHanda/atomic/internal/daemon"
	"github.com/ShriKaranHanda/atomic/internal/engine"
	"github.com/ShriKaranHanda/atomic/internal/overlay"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__runner" {
		os.Exit(overlay.RunRunnerMode(os.Args[2:]))
	}

	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := daemon.Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "atomicd failed:", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (daemon.Config, error) {
	var cfg daemon.Config
	fs := flag.NewFlagSet("atomicd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.SocketPath, "socket", daemon.DefaultSocketPath, "unix socket path")
	fs.StringVar(&cfg.StateDir, "state-dir", engine.DefaultStateDir, "state directory")
	fs.StringVar(&cfg.WorkDir, "work-dir", engine.DefaultWorkDir, "run workspace")
	fs.StringVar(&cfg.JournalDir, "journal-dir", engine.DefaultJournalDir, "journal directory")
	fs.StringVar(&cfg.RootPrefix, "root-prefix", "", "test-only root prefix")
	if err := fs.Parse(args); err != nil {
		return daemon.Config{}, err
	}
	return cfg, nil
}
