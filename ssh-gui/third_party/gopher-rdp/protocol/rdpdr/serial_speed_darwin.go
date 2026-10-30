//go:build darwin

package rdpdr

import "syscall"

func setTermiosSpeed(t *syscall.Termios, speed uint32) {
	t.Ispeed = uint64(speed)
	t.Ospeed = uint64(speed)
}

func getTermiosSpeed(t *syscall.Termios) uint32 {
	return uint32(t.Ispeed)
}
