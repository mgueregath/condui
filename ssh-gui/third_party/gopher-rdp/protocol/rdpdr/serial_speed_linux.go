//go:build linux

package rdpdr

import "syscall"

func setTermiosSpeed(t *syscall.Termios, speed uint32) {
	t.Ispeed = speed
	t.Ospeed = speed
}

func getTermiosSpeed(t *syscall.Termios) uint32 {
	return t.Ispeed
}
