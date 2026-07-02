package sessions

import (
	"io"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/osinfo"
)

type SSHSession struct {
	ID string

	Client  *ssh.Client
	Session *ssh.Session

	SFTP *sftp.Client

	RemoteOS osinfo.OSType

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool

	Rows int
	Cols int
}
