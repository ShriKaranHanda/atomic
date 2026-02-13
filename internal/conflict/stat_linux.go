//go:build linux

package conflict

import (
	"os"
	"syscall"
)

func statFields(info os.FileInfo) (uid uint32, gid uint32, inode uint64, dev uint64, ctimeNs int64, mtimeNs int64) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, 0, 0, info.ModTime().UnixNano(), info.ModTime().UnixNano()
	}
	return st.Uid, st.Gid, st.Ino, uint64(st.Dev), st.Ctim.Sec*1e9 + st.Ctim.Nsec, st.Mtim.Sec*1e9 + st.Mtim.Nsec
}
