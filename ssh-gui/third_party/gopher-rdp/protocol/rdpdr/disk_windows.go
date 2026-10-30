//go:build windows

package rdpdr

import (
	"io/fs"
	"strings"
	"syscall"
)

// isHidden reports whether the file should be marked with FILE_ATTRIBUTE_HIDDEN.
// On Windows, this checks the native hidden attribute via GetFileAttributes.
func isHidden(name string, fi fs.FileInfo) bool {
	sys := fi.Sys()
	if attr, ok := sys.(*syscall.Win32FileAttributeData); ok {
		return attr.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
	}
	// Fallback: dot-prefix convention
	return len(name) > 0 && name[0] == '.'
}

// hasPathPrefix reports whether path starts with prefix as a directory boundary.
// On Windows, paths are case-insensitive.
func hasPathPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return strings.EqualFold(path[:len(prefix)], prefix)
}
