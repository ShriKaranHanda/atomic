package preflight

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ShriKaranHanda/atomic/internal/mounts"
)

func Check() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("atomic only supports Linux runtime (got %s)", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("atomic must run as root")
	}
	if err := checkOverlaySupport(); err != nil {
		return err
	}
	if err := checkMountInfo(); err != nil {
		return err
	}
	return nil
}

func checkOverlaySupport() error {
	blob, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return fmt.Errorf("read /proc/filesystems: %w", err)
	}
	if !strings.Contains(string(blob), "overlay") {
		return fmt.Errorf("overlayfs is not available on this kernel")
	}
	return nil
}

func checkMountInfo() error {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return fmt.Errorf("open /proc/self/mountinfo: %w", err)
	}
	defer f.Close()

	parsed, err := mounts.ParseMountInfo(f)
	if err != nil {
		return err
	}
	real := mounts.WritableRealMounts(parsed)
	if len(real) == 0 {
		return fmt.Errorf("no writable real filesystems found")
	}
	return nil
}
