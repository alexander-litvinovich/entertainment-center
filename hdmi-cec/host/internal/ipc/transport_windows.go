//go:build windows

package ipc

import (
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// Address is the named pipe the host listens on.
const Address = `\\.\pipe\hdmi-cec-host`

const localUsersSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)"

// Listen opens the named pipe; go-winio creates it with FILE_PIPE_REJECT_REMOTE_CLIENTS.
func Listen(access Access) (net.Listener, error) {
	sddl := localUsersSDDL
	if access == CurrentUser {
		// A non-elevated process cannot create the service's pipe ACL.
		user, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			return nil, err
		}
		sddl = "D:P(A;;GA;;;" + user.User.Sid.String() + ")"
	}
	return winio.ListenPipe(Address, &winio.PipeConfig{SecurityDescriptor: sddl, InputBufferSize: 4096, OutputBufferSize: 4096})
}

func dial() (net.Conn, error) { return winio.DialPipe(Address, nil) }
