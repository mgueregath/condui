#!/bin/bash

BASE="ssh-gui"

mkdir -p $BASE/backend/transfers

##########################################################
# transfer_task.go
##########################################################

cat > $BASE/backend/transfers/transfer_task.go <<'EOF'
package transfers

type TransferTask struct {
	ID string `json:"id"`

	FileName string `json:"fileName"`

	Direction string `json:"direction"`

	Progress int64 `json:"progress"`

	Transferred int64 `json:"transferred"`

	Total int64 `json:"total"`

	Status string `json:"status"`
}
EOF

##########################################################
# manager.go
##########################################################

cat > $BASE/backend/transfers/manager.go <<'EOF'
package transfers

import "sync"

type Manager struct {
	mu sync.RWMutex

	tasks map[string]*TransferTask
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*TransferTask),
	}
}

func (m *Manager) Add(task *TransferTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = task
}

func (m *Manager) List() []*TransferTask {

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*TransferTask{}

	for _, t := range m.tasks {
		result = append(result, t)
	}

	return result
}
EOF

##########################################################
# upload.go
##########################################################

cat > $BASE/backend/sftp/upload.go <<'EOF'
package sftpservice

import (
	"io"
	"os"

	"github.com/pkg/sftp"
)

func UploadFile(
	client *sftp.Client,
	localPath string,
	remotePath string,
) error {

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := client.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}
EOF

##########################################################
# download.go
##########################################################

cat > $BASE/backend/sftp/download.go <<'EOF'
package sftpservice

import (
	"io"
	"os"

	"github.com/pkg/sftp"
)

func DownloadFile(
	client *sftp.Client,
	remotePath string,
	localPath string,
) error {

	src, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)

	return err
}
EOF

##########################################################
# delete.go
##########################################################

cat > $BASE/backend/sftp/delete.go <<'EOF'
package sftpservice

import "github.com/pkg/sftp"

func DeleteFile(
	client *sftp.Client,
	path string,
) error {

	return client.Remove(path)
}
EOF

##########################################################
# rename.go
##########################################################

cat > $BASE/backend/sftp/rename.go <<'EOF'
package sftpservice

import "github.com/pkg/sftp"

func RenameFile(
	client *sftp.Client,
	oldPath string,
	newPath string,
) error {

	return client.Rename(
		oldPath,
		newPath,
	)
}
EOF

##########################################################
# mkdir.go
##########################################################

cat > $BASE/backend/sftp/mkdir.go <<'EOF'
package sftpservice

import "github.com/pkg/sftp"

func CreateDirectory(
	client *sftp.Client,
	path string,
) error {

	return client.Mkdir(path)
}
EOF

echo "Incremento transferencias creado."