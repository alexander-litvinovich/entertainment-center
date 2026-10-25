//go:build !windows

package ipc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
)

// Address is the Unix socket the host listens on outside Windows (development and dry runs).
var Address = filepath.Join(os.TempDir(), "hdmi-cec-host.sock")

// Listen opens the socket for the current user only; access matters only on Windows.
func Listen(Access) (net.Listener, error) {
	if conn, err := dial(); err == nil {
		conn.Close()
		return nil, errors.New("another host is already listening on " + Address)
	}
	_ = os.Remove(Address)
	listener, err := net.Listen("unix", Address)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(Address, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func dial() (net.Conn, error) { return net.Dial("unix", Address) }
