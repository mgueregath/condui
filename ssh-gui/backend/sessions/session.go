package sessions

import (
	"io"
	"os"
	"os/exec"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/osinfo"
)

type SSHSession struct {
	ID string

	Client  *ssh.Client
	Session *ssh.Session
	Command *exec.Cmd
	Pty     *os.File

	SFTP *sftp.Client

	RemoteOS osinfo.OSType

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool
	IsLocal   bool

	Rows int
	Cols int
}
