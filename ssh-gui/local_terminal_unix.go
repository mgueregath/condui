//go:build !windows

package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"

	"ssh-gui/backend/sessions"
)

type unixLocalTerminal struct {
	file *os.File
	cmd  *exec.Cmd
}

func startLocalTerminal(cols, rows int) (sessions.LocalTerminal, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-i")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	file, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, err
	}

	return &unixLocalTerminal{file: file, cmd: cmd}, nil
}

func (terminal *unixLocalTerminal) Read(data []byte) (int, error) {
	return terminal.file.Read(data)
}

func (terminal *unixLocalTerminal) Write(data []byte) (int, error) {
	return terminal.file.Write(data)
}

func (terminal *unixLocalTerminal) Close() error {
	if terminal.cmd.Process != nil {
		_ = terminal.cmd.Process.Kill()
	}
	return terminal.file.Close()
}

func (terminal *unixLocalTerminal) Resize(cols, rows int) error {
	return pty.Setsize(terminal.file, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (terminal *unixLocalTerminal) Wait() error {
	return terminal.cmd.Wait()
}
