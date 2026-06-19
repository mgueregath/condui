package sessions

import (
	"io"

	"golang.org/x/crypto/ssh"
	"github.com/pkg/sftp"
)

type SSHSession struct {
	ID string

	Client  *ssh.Client
	Session *ssh.Session

	SFTP *sftp.Client

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool

	Rows int
	Cols int
}
