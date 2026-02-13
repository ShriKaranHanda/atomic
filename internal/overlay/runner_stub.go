//go:build !linux

package overlay

import (
	"fmt"
	"os"
)

func RunRunnerMode(args []string) int {
	fmt.Fprintln(os.Stderr, "runner mode is only supported on Linux")
	return 2
}
