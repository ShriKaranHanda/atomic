package mounts

import (
	"strings"
	"testing"
)

func TestParseMountInfo(t *testing.T) {
	input := strings.NewReader(`34 2 253:1 / / rw,relatime shared:1 - ext4 /dev/vda1 rw,discard
46 34 0:37 / /tmp rw,nosuid,nodev shared:19 - tmpfs tmpfs rw
50 34 0:45 / /Users\\040Name ro,relatime shared:53 - virtiofs mount0 ro
`)

	mounts, err := ParseMountInfo(input)
	if err != nil {
		t.Fatalf("ParseMountInfo returned error: %v", err)
	}
	if len(mounts) != 3 {
		t.Fatalf("expected 3 mounts, got %d", len(mounts))
	}
	if mounts[2].MountPoint != "/Users Name" {
		t.Fatalf("expected escaped mountpoint to be decoded, got %q", mounts[2].MountPoint)
	}
}

func TestWritableRealMounts(t *testing.T) {
	input := strings.NewReader(`34 2 253:1 / / rw,relatime shared:1 - ext4 /dev/vda1 rw
46 34 0:37 / /tmp rw,nosuid,nodev shared:19 - tmpfs tmpfs rw
56 34 253:13 / /boot rw,relatime shared:96 - ext4 /dev/vda13 rw
29 34 0:26 / /proc rw,nosuid,nodev,noexec,relatime shared:12 - proc proc rw
`)

	mounts, err := ParseMountInfo(input)
	if err != nil {
		t.Fatalf("ParseMountInfo returned error: %v", err)
	}
	real := WritableRealMounts(mounts)
	if len(real) != 2 {
		t.Fatalf("expected 2 writable real mounts, got %d", len(real))
	}
	if real[0].MountPoint != "/" || real[1].MountPoint != "/boot" {
		t.Fatalf("unexpected real mount order: %#v", real)
	}
}
