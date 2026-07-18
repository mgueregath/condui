//go:build windows

package main

import (
	"context"
	"os"

	"github.com/UserExistsError/conpty"

	"ssh-gui/backend/sessions"
)

type windowsLocalTerminal struct {
	terminal *conpty.ConPty
}

func startLocalTerminal(cols, rows int) (sessions.LocalTerminal, error) {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}

	terminal, err := conpty.Start(
		shell,
		conpty.ConPtyDimensions(cols, rows),
		conpty.ConPtyEnv(append(os.Environ(), "TERM=xterm-256color")),
	)
	if err != nil {
		return nil, err
	}

	return &windowsLocalTerminal{terminal: terminal}, nil
}

func (terminal *windowsLocalTerminal) Read(data []byte) (int, error) {
	return terminal.terminal.Read(data)
}

func (terminal *windowsLocalTerminal) Write(data []byte) (int, error) {
	return terminal.terminal.Write(data)
}

func (terminal *windowsLocalTerminal) Close() error {
	return terminal.terminal.Close()
}

func (terminal *windowsLocalTerminal) Resize(cols, rows int) error {
	return terminal.terminal.Resize(cols, rows)
}

func (terminal *windowsLocalTerminal) Wait() error {
	_, err := terminal.terminal.Wait(context.Background())
	return err
}
