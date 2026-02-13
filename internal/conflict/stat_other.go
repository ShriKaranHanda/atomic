//go:build !linux

package conflict

import "os"

func statFields(info os.FileInfo) (uid uint32, gid uint32, inode uint64, dev uint64, ctimeNs int64, mtimeNs int64) {
	ns := info.ModTime().UnixNano()
	return 0, 0, 0, 0, ns, ns
}
