package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShriKaranHanda/atomic/internal/mounts"
)

type MountSpec struct {
	MountPoint string `json:"mount_point"`
	LowerDir   string `json:"lower_dir"`
	UpperDir   string `json:"upper_dir"`
	WorkDir    string `json:"work_dir"`
}

type RunnerSpec struct {
	MergedDir    string      `json:"merged_dir"`
	RootLowerDir string      `json:"root_lower_dir"`
	RootUpperDir string      `json:"root_upper_dir"`
	RootWorkDir  string      `json:"root_work_dir"`
	ExtraMounts  []MountSpec `json:"extra_mounts"`
	CWD          string      `json:"cwd"`
	ScriptPath   string      `json:"script_path"`
	ScriptArgs   []string    `json:"script_args"`
	RunAsUID     uint32      `json:"run_as_uid"`
	RunAsGID     uint32      `json:"run_as_gid"`
}

type RunConfig struct {
	RunID      string
	WorkRoot   string
	ScriptPath string
	ScriptArgs []string
	CWD        string
	RunAsUID   uint32
	RunAsGID   uint32
	Verbose    bool
	Stdout     io.Writer
	Stderr     io.Writer
	Stdin      io.Reader
}

type RunResult struct {
	ExitCode   int
	RunDir     string
	UpperDirs  []MountSpec
	MergedDir  string
	StartedAt  time.Time
	FinishedAt time.Time
}

func DiscoverMounts() ([]MountSpec, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("open /proc/self/mountinfo: %w", err)
	}
	defer f.Close()
	parsed, err := mounts.ParseMountInfo(f)
	if err != nil {
		return nil, err
	}
	real := mounts.WritableRealMounts(parsed)
	if len(real) == 0 {
		return nil, errors.New("no writable mounts found")
	}
	result := make([]MountSpec, 0, len(real))
	for _, mount := range real {
		if !overlayLowerSupported(mount.FSType) {
			continue
		}
		result = append(result, MountSpec{MountPoint: mount.MountPoint, LowerDir: mount.MountPoint})
	}
	if len(result) == 0 {
		return nil, errors.New("no overlay-compatible writable mounts found")
	}
	sort.Slice(result, func(i, j int) bool {
		return depth(result[i].MountPoint) < depth(result[j].MountPoint)
	})
	return result, nil
}

func overlayLowerSupported(fsType string) bool {
	switch fsType {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "f2fs":
		return true
	default:
		return false
	}
}

func RunScript(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	if cfg.WorkRoot == "" {
		return nil, errors.New("work root is required")
	}
	if cfg.ScriptPath == "" {
		return nil, errors.New("script path is required")
	}
	if cfg.CWD == "" {
		cfg.CWD = "/"
	}
	mountSpecs, err := DiscoverMounts()
	if err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC()
	runDir := filepath.Join(cfg.WorkRoot, cfg.RunID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}
	mergedDir := filepath.Join(runDir, "merged")
	if err := os.MkdirAll(mergedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create merged directory: %w", err)
	}

	rootUpper := filepath.Join(runDir, "upper-root")
	rootWork := filepath.Join(runDir, "work-root")
	if err := os.MkdirAll(rootUpper, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rootWork, 0o700); err != nil {
		return nil, err
	}

	extraMounts := make([]MountSpec, 0, len(mountSpecs))
	for _, mount := range mountSpecs {
		if mount.MountPoint == "/" {
			continue
		}
		name := sanitizeMountName(mount.MountPoint)
		upper := filepath.Join(runDir, "upper", name)
		work := filepath.Join(runDir, "work", name)
		if err := os.MkdirAll(upper, 0o755); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(work, 0o700); err != nil {
			return nil, err
		}
		extraMounts = append(extraMounts, MountSpec{MountPoint: mount.MountPoint, LowerDir: mount.MountPoint, UpperDir: upper, WorkDir: work})
	}

	spec := RunnerSpec{
		MergedDir:    mergedDir,
		RootLowerDir: "/",
		RootUpperDir: rootUpper,
		RootWorkDir:  rootWork,
		ExtraMounts:  extraMounts,
		CWD:          cfg.CWD,
		ScriptPath:   cfg.ScriptPath,
		ScriptArgs:   cfg.ScriptArgs,
		RunAsUID:     cfg.RunAsUID,
		RunAsGID:     cfg.RunAsGID,
	}
	specPath := filepath.Join(runDir, "runner-spec.json")
	if err := writeSpec(specPath, spec); err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "unshare", "--mount", "--propagation", "private", "--fork", "--", exe, "__runner", "--spec", specPath)
	if cfg.Stdout != nil {
		cmd.Stdout = cfg.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if cfg.Stderr != nil {
		cmd.Stderr = cfg.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	if cfg.Stdin != nil {
		cmd.Stdin = cfg.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("run unshare: %w", err)
		}
	}
	allUpper := []MountSpec{{MountPoint: "/", LowerDir: "/", UpperDir: rootUpper, WorkDir: rootWork}}
	allUpper = append(allUpper, extraMounts...)
	return &RunResult{
		ExitCode:   exitCode,
		RunDir:     runDir,
		UpperDirs:  allUpper,
		MergedDir:  mergedDir,
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}, nil
}

func writeSpec(path string, spec RunnerSpec) error {
	blob, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o600)
}

func sanitizeMountName(mountPoint string) string {
	if mountPoint == "/" {
		return "root"
	}
	name := strings.Trim(mountPoint, "/")
	name = strings.ReplaceAll(name, "/", "__")
	if name == "" {
		name = "root"
	}
	return name
}

func depth(path string) int {
	if path == "/" {
		return 0
	}
	return len(strings.Split(strings.Trim(path, "/"), "/"))
}
