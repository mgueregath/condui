package sessions

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	ID string

	Client  *ssh.Client
	Session *ssh.Session

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool

	Rows int
	Cols int
}
