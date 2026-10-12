package sessions

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/osinfo"
	"ssh-gui/backend/systeminfo"
)

type SSHSession struct {
	ID string

	Client        *ssh.Client
	Session       *ssh.Session
	Command       *exec.Cmd
	Pty           *os.File
	LocalTerminal LocalTerminal

	SFTP *sftp.Client

	RemoteOS osinfo.OSType

	SystemStatsMu    sync.Mutex
	SystemPartitions systeminfo.PartitionCache

	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader

	Connected bool
	IsLocal   bool

	Rows int
	Cols int

	// SudoPassword stores the sudo password for this session (if needed)
	SudoPassword string
}

type LocalTerminal interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	Wait() error
}
