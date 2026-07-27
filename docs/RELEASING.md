# Releasing

Pushing a tag matching `vX.Y.Z` triggers [`.github/workflows/release.yml`](../.github/workflows/release.yml),
which builds the desktop app for macOS, Windows and Linux and publishes a GitHub Release with all
artifacts attached.

```bash
git tag v1.0.0
git push origin v1.0.0
```

This produces:

- `Condui-mac.dmg` — signed and notarized universal (arm64 + amd64) build
- `Condui-windows-x64.exe` — NSIS installer (unsigned until SignPath is approved, see below)
- `Condui-linux-x64.AppImage`, `Condui-linux-x64.deb`, `Condui-linux-x64.rpm`

The public build never includes the private `ssh-gui/backend/dbexplorer` (navoro) submodule or the
`dbmanager` Go build tag — the workflow checks out the repo without submodules, matching how an outside
contributor would build the OSS repo.

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
   automatically submits `Condui-windows-x64.exe` to SignPath, waits for the signed artifact, and ships
   that instead of the unsigned one. No workflow changes needed.

## Notes

- `ssh-gui/build/linux/nfpm/nfpm.yaml` and `ssh-gui/build/windows/nsis/project.nsi` had never been
  customized from the Wails scaffold defaults (`testapp`, "My Company") — this was fixed as part of
  setting up this pipeline so the Linux packages and Windows installer/product name now correctly say
  "Condui".
- To test the pipeline without cutting a real release, push a tag to a fork or a throwaway
  `vX.Y.Z-rc1` tag on a branch you don't mind deleting afterward — there's no manual `workflow_dispatch`
  trigger by design, only tag pushes.
