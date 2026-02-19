//go:build linux

package overlay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

func RunRunnerMode(args []string) int {
	specPath, err := parseRunnerArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner argument error:", err)
		return 2
	}
	spec, err := loadSpec(specPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runner spec error:", err)
		return 2
	}
	return runInNamespace(spec)
}

func parseRunnerArgs(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--spec" && i+1 < len(args) {
			return args[i+1], nil
		}
	}
	return "", errors.New("missing --spec")
}

func loadSpec(path string) (*RunnerSpec, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec RunnerSpec
	if err := json.Unmarshal(blob, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func runInNamespace(spec *RunnerSpec) int {
	if err := os.MkdirAll(spec.MergedDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "create merged dir:", err)
		return 2
	}
	mounted := []string{}
	mountOne := func(target, lower, upper, work string) error {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(upper, 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(work, 0o700); err != nil {
			return err
		}
		data := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
		if err := syscall.Mount("overlay", target, "overlay", 0, data); err != nil {
			return fmt.Errorf("mount overlay at %s: %w", target, err)
		}
		mounted = append(mounted, target)
		return nil
	}
	cleanup := func() {
		for i := len(mounted) - 1; i >= 0; i-- {
			_ = syscall.Unmount(mounted[i], syscall.MNT_DETACH)
		}
	}
	defer cleanup()

	if err := mountOne(spec.MergedDir, spec.RootLowerDir, spec.RootUpperDir, spec.RootWorkDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	sort.Slice(spec.ExtraMounts, func(i, j int) bool {
		return depth(spec.ExtraMounts[i].MountPoint) < depth(spec.ExtraMounts[j].MountPoint)
	})
	for _, m := range spec.ExtraMounts {
		target := filepath.Join(spec.MergedDir, strings.TrimPrefix(filepath.Clean(m.MountPoint), "/"))
		if err := mountOne(target, m.LowerDir, m.UpperDir, m.WorkDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}

	cmdArgs := []string{
		spec.MergedDir,
		"/usr/bin/setpriv",
		fmt.Sprintf("--reuid=%d", spec.RunAsUID),
		fmt.Sprintf("--regid=%d", spec.RunAsGID),
		"--clear-groups",
		"/bin/bash",
		"-ceu",
		`cd "$1"; shift; exec /bin/bash "$@"`,
		"atomic",
		spec.CWD,
		spec.ScriptPath,
	}
	cmdArgs = append(cmdArgs, spec.ScriptArgs...)
	cmd := exec.Command("chroot", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	fmt.Fprintln(os.Stderr, "failed to execute script:", err)
	return 2
}
