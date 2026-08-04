//go:build !production

package storage

// appDirName is the per-OS config subdirectory name for non-production
// builds (dev server, `task run`/`task build` without the production tag —
// see build/config.yml dev_mode.executes and build/{darwin,windows,linux}/Taskfile.yml).
// Deliberately separate from the release build's "Condui" directory so a dev
// build never opens (and potentially corrupts, via concurrent SQLite writes
// from two independent processes) the same condui.db a locally installed
// production build is using.
const appDirName = "Condui-dev"
