//go:build production

package storage

// appDirName is the per-OS config subdirectory name for release builds
// (built with -tags production, see build/{darwin,windows,linux}/Taskfile.yml).
// Kept as "Condui" (unchanged) so existing installs keep using their data.
const appDirName = "Condui"
