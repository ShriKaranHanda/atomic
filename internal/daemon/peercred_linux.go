//go:build linux

package daemon

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

func peerCredentials(conn net.Conn) (uint32, uint32, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, 0, fmt.Errorf("expected unix connection, got %T", conn)
	}
	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return 0, 0, err
	}
	var uid uint32
	var gid uint32
	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		cred, err := getsockoptUcred(int(fd))
		if err != nil {
			controlErr = err
			return
		}
		uid = cred.Uid
		gid = cred.Gid
	}); err != nil {
		return 0, 0, err
	}
	if controlErr != nil {
		return 0, 0, controlErr
	}
	return uid, gid, nil
}

func getsockoptUcred(fd int) (*syscall.Ucred, error) {
	var cred syscall.Ucred
	size := uint32(unsafe.Sizeof(cred))
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.SOL_SOCKET),
		uintptr(syscall.SO_PEERCRED),
		uintptr(unsafe.Pointer(&cred)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return &cred, nil
}
