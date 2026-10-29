//go:build android

package main

import (
	"bytes"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	// Android defines no HOME, and the Go runtime of a c-shared library only sees
	// the environment the loader handed it, so the value must be set from Go.
	// Storage, vault and known_hosts all derive their paths from it.
	if os.Getenv("HOME") == "" {
		if cmd, err := os.ReadFile("/proc/self/cmdline"); err == nil {
			pkg, _, _ := bytes.Cut(cmd, []byte{0})
			dir := "/data/data/" + string(pkg) + "/files"
			if os.MkdirAll(dir, 0o700) == nil {
				os.Setenv("HOME", dir)
			}
		}
	}

	// Register main function to be called when the Android app initializes
	// This is necessary because in c-shared build mode, main() is not automatically called
	application.RegisterAndroidMain(main)
}
