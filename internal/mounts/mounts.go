package mounts

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Mount struct {
	MountPoint string
	FSType     string
	Options    map[string]bool
	Source     string
}

func ParseMountInfo(r io.Reader) ([]Mount, error) {
	sc := bufio.NewScanner(r)
	var out []Mount
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		leftRight := strings.SplitN(line, " - ", 2)
		if len(leftRight) != 2 {
			return nil, fmt.Errorf("invalid mountinfo line: %q", line)
		}
		leftFields := strings.Fields(leftRight[0])
		rightFields := strings.Fields(leftRight[1])
		if len(leftFields) < 6 || len(rightFields) < 2 {
			return nil, fmt.Errorf("invalid mountinfo fields: %q", line)
		}
		mountPoint := decodeEscapes(leftFields[4])
		opts := parseOptions(leftFields[5])
		mount := Mount{
			MountPoint: mountPoint,
			Options:    opts,
			FSType:     rightFields[0],
		}
		if len(rightFields) > 1 {
			mount.Source = rightFields[1]
		}
		out = append(out, mount)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan mountinfo: %w", err)
	}
	return out, nil
}

func WritableRealMounts(mounts []Mount) []Mount {
	out := make([]Mount, 0, len(mounts))
	for _, m := range mounts {
		if m.Options["ro"] {
			continue
		}
		if IsPseudoFS(m.FSType) {
			continue
		}
		if m.FSType == "overlay" {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MountPoint == out[j].MountPoint {
			return out[i].FSType < out[j].FSType
		}
		if pathDepth(out[i].MountPoint) == pathDepth(out[j].MountPoint) {
			return out[i].MountPoint < out[j].MountPoint
		}
		return pathDepth(out[i].MountPoint) < pathDepth(out[j].MountPoint)
	})
	return out
}

func IsPseudoFS(fsType string) bool {
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "tmpfs", "securityfs", "mqueue", "pstore", "tracefs", "debugfs", "autofs", "efivarfs", "hugetlbfs", "fusectl", "configfs", "binfmt_misc", "nsfs", "ramfs", "selinuxfs", "bpf":
		return true
	default:
		return false
	}
}

func parseOptions(raw string) map[string]bool {
	out := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out[item] = true
	}
	return out
}

func decodeEscapes(s string) string {
	replacer := strings.NewReplacer(`\\040`, " ", `\\011`, "\t", `\\012`, "\n", `\\134`, `\\`)
	return replacer.Replace(s)
}

func pathDepth(path string) int {
	if path == "/" {
		return 0
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts)
}
