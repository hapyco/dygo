# Installation

dygo release binaries are distributed through GitHub Releases.

The intended public install command is:

```sh
curl -fsSL https://dygo.dev/install | sh
```

Until `dygo.dev/install` is wired, use the repository-hosted installer:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
```

The installer places the managed binary in:

```txt
~/.dygo/bin
```

If that directory is not on `PATH`, the installer prints the shell profile line to add.

## Options

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | DYGO_VERSION=v0.1.0 sh
```

Install somewhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | DYGO_INSTALL_DIR=/usr/local/bin sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.ps1 | iex
```

## Upgrades

Update the dygo binary out of band with the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
```

Inside a generated dygo project, `dygo upgrade` updates the project `go.mod` dygo dependency, dygo-managed generated runner files, and the cached Studio UI assets when the target dygo version differs from the project version. Project upgrades refuse dirty git worktrees.

Useful upgrade modes:

```sh
dygo upgrade --check
dygo upgrade --dry-run
dygo upgrade --to v0.1.0
dygo upgrade --yes
```
