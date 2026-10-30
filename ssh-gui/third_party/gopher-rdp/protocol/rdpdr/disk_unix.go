//go:build !windows

package rdpdr

import (
	"io/fs"
	"strings"
)

// isHidden reports whether the file should be marked with FILE_ATTRIBUTE_HIDDEN.
// On Unix, files starting with '.' are conventionally hidden.
func isHidden(name string, _ fs.FileInfo) bool {
	return len(name) > 0 && name[0] == '.'
}

// hasPathPrefix reports whether path starts with prefix as a directory boundary.
// On Unix, paths are case-sensitive.
func hasPathPrefix(path, prefix string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
