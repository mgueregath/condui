# Releasing

Pushing a tag matching `vX.Y.Z` triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which builds the desktop app for macOS, Windows and Linux and publishes a GitHub Release with all
artifacts attached.

```bash
git tag v1.0.0
git push origin v1.0.0
```

This produces:

**Fresh-install artifacts** (what a user downloads from the Releases page the first time):
- `Condui-mac.dmg` — signed and notarized universal (arm64 + amd64) build
- `Condui-windows-x64-installer.exe` — NSIS installer (unsigned until SignPath is approved, see below)
- `Condui-linux-x64.AppImage`, `Condui-linux-x64.deb`, `Condui-linux-x64.rpm`

**In-app updater artifacts** (what an already-installed Condui downloads via Account → Check for
Updates; see [Updating](#updating) below):
- `Condui-darwin-arm64-amd64.app.zip` — the same signed/notarized `.app` bundle, zipped
- `Condui-windows-amd64-update.exe` — the bare `.exe` (no installer wrapper)
- `Condui-linux-x64.AppImage` — reused as-is; it's already a swappable single-file binary
- `SHA256SUMS` — digests the updater verifies downloads against

The public build never includes the private `ssh-gui/backend/dbexplorer` (navoro) submodule or the
`dbmanager` Go build tag — the workflow checks out the repo without submodules, matching how an outside
contributor would build the OSS repo.

## Updating

Condui embeds [Wails v3's built-in updater](https://v3.wails.io/tutorials/04-self-update-a-wails-app/)
(`github.com/wailsapp/wails/v3/pkg/updater`, already vendored at the pinned `go.mod` version — no extra
dependency). `ssh-gui/main.go` wires it to the `mgueregath/condui` GitHub Releases API
(`backend/buildconfig/build.config.yaml`, key `update_repo`) with the release's `SHA256SUMS` as the
integrity check (`github.Config.ChecksumAsset`). The version compared against `tag_name` is baked in at
build time via `-ldflags -X main.currentVersion=...` (see the `VERSION` var in each
`ssh-gui/build/{darwin,windows,linux}/Taskfile.yml`, sourced from `git describe --tags`).

Users trigger a check from the Account modal ("Check for Updates…" — `ssh-gui/app_update.go`,
`CheckForUpdates`), which opens the framework's built-in update window: it shows the release notes,
downloads and verifies the matching asset, and swaps it in on **Restart & Apply**.

Why the updater needs its own assets instead of reusing the DMG/installer:
- The updater only knows how to unpack `.zip`/`.tar.gz` (or take a bare file) before swapping — it can't
  mount a `.dmg` or invoke an NSIS installer. Hence the separate zipped `.app` and bare `.exe`.
- The default asset matcher picks a release asset by `GOOS`/`GOARCH` substring in the filename (`darwin`,
  `windows`, `linux`, `amd64`, `arm64` — with `x64`/`x86_64`/`aarch64` aliases). The macOS zip is named
  with **both** arch tokens since it's a universal binary that needs to match either host arch.
- The AppImage needs no separate asset: it's already a bare, swappable single-file binary, and its
  existing name (`Condui-linux-x64.AppImage`) already contains matcher-recognized tokens (`linux`, `x64`
  as an `amd64` alias).
- `.deb`/`.rpm` have no auto-update path here — package-manager artifacts aren't something this updater
  (or any equivalent without a hosted APT/YUM repo) can swap in place. Users on those tracks reinstall the
  new `.deb`/`.rpm` manually, same as before this feature existed.
- The NSIS installer, `.deb` and `.rpm` all contain the same platform/arch filename tokens as their
  corresponding update asset (e.g. `Condui-windows-x64-installer.exe` and
  `Condui-windows-amd64-update.exe` both match `windows`+`amd64`), so the default matcher alone can't tell
  them apart — it just returns whichever comes first in the release's asset list. `main.go` wires a custom
  `AssetMatcher` (`updateAssetMatcher`) that filters out anything with `installer`, `.deb`, or `.rpm` in
  the name before delegating to `github.DefaultAssetMatcher`, so the updater can only ever pick a real
  update asset.

The Windows portable `.exe` ships unsigned for now, same as the installer (see SignPath section below) —
signing it requires a second SignPath Artifact Configuration once that's set up.

For stronger tamper-resistance than the `SHA256SUMS` digest check (integrity, not authenticity), the
Updater guide covers adding an Ed25519 signature (`updater.Config.PublicKey` + a signing step in CI) — not
wired up here; revisit if the distribution channel's trust model changes.

## Required secrets (macOS signing & notarization)

Set these under **Settings → Secrets and variables → Actions** on the `condui` repo:

| Secret | Value |
| --- | --- |
| `MACOS_CERTIFICATE_P12` | `base64 -i DeveloperIDApplication.p12 \| pbcopy` output of your exported Developer ID Application certificate |
| `MACOS_CERTIFICATE_PASSWORD` | Password used when exporting the `.p12` |
| `MACOS_KEYCHAIN_PASSWORD` | Any password — only used to protect the throwaway CI keychain for the run |
| `MACOS_SIGN_IDENTITY` | Exact identity string, e.g. `Developer ID Application: Mirko Gueregat (TEAMID)` |
| `APPLE_ID` | Apple ID used for notarization |
| `APPLE_TEAM_ID` | Apple Developer Team ID |
| `APPLE_APP_SPECIFIC_PASSWORD` | App-specific password (generate at appleid.apple.com) for `notarytool` |

Without these, `build-macos` fails at the "Import signing certificate" step. Local ad-hoc releases via
`scripts/release-macos.sh` are unaffected — they keep using `.env` + the local keychain profile from
`scripts/setup-macos-signing.sh`.

## Windows: applying for free code signing (SignPath Foundation)

The Windows installer currently ships **unsigned**. [SignPath Foundation](https://signpath.org) signs
open-source projects for free (no personal ID needed — they verify the binary was built from the public
`condui` repo instead). Steps to enable it:

1. Apply at https://signpath.org/apply. Requirements: public repo, an existing automated build (this
   workflow now qualifies), and an actual release history — so it's worth doing an initial unsigned
   release first.
2. Once approved, in the SignPath dashboard:
   - Add **GitHub.com** as a Trusted Build System and link it to the `mgueregath/condui` repo/workflow.
   - Create a **Project** (e.g. `condui`) and an **Artifact Configuration** describing the `.exe` to sign.
   - Create two **Signing Policies**: `test-signing` (any branch, for validation) and `release-signing`
     (restricted to tag pushes / protected refs, used for real releases).
3. Add these repo secrets:
   - `SIGNPATH_API_TOKEN`
   - `SIGNPATH_ORGANIZATION_ID`
   - `SIGNPATH_PROJECT_SLUG`
   - `SIGNPATH_SIGNING_POLICY_SLUG`
4. That's it — `.github/workflows/release.yml` already has the signing steps in the `build-windows` job,
   gated on `secrets.SIGNPATH_API_TOKEN != ''`. As soon as the secrets exist, the next tagged release
   automatically submits `Condui-windows-x64-installer.exe` to SignPath, waits for the signed artifact,
   and ships that instead of the unsigned one. No workflow changes needed.

## Notes

- `ssh-gui/build/linux/nfpm/nfpm.yaml` and `ssh-gui/build/windows/nsis/project.nsi` had never been
  customized from the Wails scaffold defaults (`testapp`, "My Company") — this was fixed as part of
  setting up this pipeline so the Linux packages and Windows installer/product name now correctly say
  "Condui".
- To test the pipeline without cutting a real release, push a tag to a fork or a throwaway
  `vX.Y.Z-rc1` tag on a branch you don't mind deleting afterward — there's no manual `workflow_dispatch`
  trigger by design, only tag pushes.
