//go:build linux

package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPeerCredentials(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "peercred.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close()

	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(ready)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		uid, gid, err := peerCredentials(conn)
		if err != nil {
			t.Errorf("peerCredentials returned error: %v", err)
			return
		}
		if uid != uint32(os.Geteuid()) || gid != uint32(os.Getegid()) {
			t.Errorf("unexpected peer creds uid=%d gid=%d", uid, gid)
		}
	}()
	<-ready

	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial unix: %v", err)
	}
	defer c.Close()
	<-done
}
