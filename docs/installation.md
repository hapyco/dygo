# Installation

dygo release binaries are distributed through [GitHub Releases](https://github.com/hapyco/dygo/releases). Releases include macOS, Linux, and Windows binaries for AMD64 and ARM64.

## Install And Verify

Install the latest stable release on macOS or Linux:

```sh
curl -fsSL https://dygo.dev/install | sh
```

The installer places the managed binary at `~/.dygo/bin/dygo`. It prints the shell profile line to add if that directory is not on `PATH`:

```sh
export PATH="$HOME/.dygo/bin:$PATH"
dygo version
command -v dygo
```

Windows PowerShell:

```powershell
irm https://dygo.dev/install.ps1 | iex
$env:PATH = "$HOME\.dygo\bin;$env:PATH"
dygo version
(Get-Command dygo).Source
```

The Windows binary is `$HOME\.dygo\bin\dygo.exe`. Add its directory to your user `PATH` to make it available in future terminals. The command above updates `PATH` in the current terminal only.

The installers need network access to GitHub. The POSIX installer also needs `curl`, `tar`, `install`, and a SHA-256 command (`sha256sum` or `shasum`). An installation directory must be writable by the current user.

## Pin A Version

Use a release-hosted installer and set `DYGO_VERSION` to the same release. For example, to install v0.0.6:

```sh
version=v0.0.6
curl -fsSL "https://github.com/hapyco/dygo/releases/download/$version/install.sh" | DYGO_VERSION="$version" sh
```

Windows PowerShell:

```powershell
$env:DYGO_VERSION = "v0.0.6"
irm "https://github.com/hapyco/dygo/releases/download/$env:DYGO_VERSION/install.ps1" | iex
Remove-Item Env:DYGO_VERSION
```

Use the required version in both the URL and the environment variable. A prerelease must be selected explicitly. The default `latest` selection uses GitHub's latest stable release.

Set `DYGO_INSTALL_DIR` to change the managed directory:

```sh
curl -fsSL https://dygo.dev/install | DYGO_INSTALL_DIR="$HOME/bin" sh
```

In PowerShell, set `$env:DYGO_INSTALL_DIR` before running the installer. Remove that environment variable afterward if the override is temporary.

## Update Or Downgrade The CLI

Run the installer again to update the global CLI to the latest stable release:

```sh
curl -fsSL https://dygo.dev/install | DYGO_VERSION=latest sh
dygo version
```

Windows PowerShell:

```powershell
$env:DYGO_VERSION = "latest"
irm https://dygo.dev/install.ps1 | iex
Remove-Item Env:DYGO_VERSION
dygo version
```

To update or downgrade to a particular release, use the pinned installation commands above with that release. Check `dygo version` and the binary location after the change. If your shell resolves another copy of dygo, put the managed directory first on `PATH`.

Before replacement, the installers verify the archive's SHA-256 checksum and require the candidate binary's `version` command to succeed and report the requested version. They stage the candidate in the installation directory and replace the managed binary atomically. Download, checksum, version, and replacement failures preserve the previous binary. Retry the installer after resolving the reported error. On Windows, close processes that hold the binary open if replacement reports that the file is in use.

Installation phase messages go to stderr. The POSIX installer uses a delayed spinner when stderr is an interactive terminal. Redirected output, CI, and `TERM=dumb` use plain messages. Cancelling the POSIX installer stops the operation and removes temporary files.

## Upgrade A Generated Project

First install the CLI version you want the project to use. Inside the generated project, inspect and apply the project upgrade:

```sh
dygo version
dygo upgrade --check
dygo upgrade --dry-run
dygo upgrade --yes
dygo db migrate
```

`--check` compares the project's dygo dependency with the running binary's version. It does not discover the latest release or change files. `--dry-run` plans the project changes without applying them. The optional `--to` value must match the running CLI version; it does not download another CLI.

When the project version differs, `dygo upgrade` updates the `go.mod` dependency, dygo-managed generated runner files, tracked Core and Studio metadata, and cached Studio UI assets. Applying an upgrade refuses a dirty git worktree. Commit or stash project changes first. After metadata changes, run `dygo db migrate` before starting the project to apply additive database changes.

Maintainers should use the [Release Process](releasing.md) to build, tag, and publish framework releases.
