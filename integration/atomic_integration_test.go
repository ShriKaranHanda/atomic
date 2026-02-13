//go:build linux

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var atomicBin string

func TestMain(m *testing.M) {
	if os.Getenv("ATOMIC_INTEGRATION") != "1" {
		os.Exit(0)
	}
	if os.Geteuid() != 0 {
		os.Stderr.WriteString("integration tests must run as root\n")
		os.Exit(1)
	}

	tmp, err := os.MkdirTemp("", "atomic-bin-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)
	atomicBin = filepath.Join(tmp, "atomic")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	repoRoot := filepath.Dir(wd)
	build := exec.Command("go", "build", "-o", atomicBin, "./cmd/atomic")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func TestSuccessCommitsChanges(t *testing.T) {
	work := integrationWorkDir(t)
	target := filepath.Join(work, "target.txt")
	script := filepath.Join(work, "ok.sh")
	content := "#!/usr/bin/env bash\nset -euo pipefail\necho committed >" + target + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command(atomicBin, script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("atomic run failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read committed file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "committed" {
		t.Fatalf("unexpected committed content: %q", data)
	}
}

func TestFailureDoesNotCommit(t *testing.T) {
	work := integrationWorkDir(t)
	target := filepath.Join(work, "target.txt")
	script := filepath.Join(work, "fail.sh")
	content := "#!/usr/bin/env bash\nset -euo pipefail\necho transient >" + target + "\nexit 1\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command(atomicBin, script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected atomic run to fail")
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 10 {
		t.Fatalf("expected exit code 10, got %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected target to not exist, stat err=%v", err)
	}
}

func TestDeleteCommits(t *testing.T) {
	work := integrationWorkDir(t)
	target := filepath.Join(work, "delete-me.txt")
	if err := os.WriteFile(target, []byte("bye"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	script := filepath.Join(work, "del.sh")
	content := "#!/usr/bin/env bash\nset -euo pipefail\nrm -f " + target + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command(atomicBin, script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("atomic run failed: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, stat err=%v", err)
	}
}

func TestConflictRejected(t *testing.T) {
	work := integrationWorkDir(t)
	target := filepath.Join(work, "race.txt")
	script := filepath.Join(work, "race.sh")
	content := "#!/usr/bin/env bash\nset -euo pipefail\necho atomic >" + target + "\nsleep 2\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command(atomicBin, script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start atomic: %v", err)
	}

	time.Sleep(600 * time.Millisecond)
	if err := os.WriteFile(target, []byte("external"), 0o644); err != nil {
		t.Fatalf("write external conflict file: %v", err)
	}

	err := cmd.Wait()
	if err == nil {
		t.Fatalf("expected conflict failure")
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 21 {
		t.Fatalf("expected exit code 21, got %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if strings.TrimSpace(string(data)) != "external" {
		t.Fatalf("expected external content to remain, got %q", data)
	}
}

func integrationWorkDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll("/var/lib/atomic-it", 0o755); err != nil {
		t.Fatalf("create integration parent dir: %v", err)
	}
	dir, err := os.MkdirTemp("/var/lib/atomic-it", "case-")
	if err != nil {
		t.Fatalf("create integration work dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
