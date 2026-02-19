package main

import (
	"os"

	"github.com/ShriKaranHanda/atomic/internal/cli"
	"github.com/ShriKaranHanda/atomic/internal/overlay"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__runner" {
		os.Exit(overlay.RunRunnerMode(os.Args[2:]))
	}
	os.Exit(cli.Run(os.Args[1:]))
}
