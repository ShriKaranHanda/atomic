//go:build !linux

package daemon

import (
	"fmt"
	"net"
)

func peerCredentials(conn net.Conn) (uint32, uint32, error) {
	return 0, 0, fmt.Errorf("peer credential lookup is only supported on Linux")
}
