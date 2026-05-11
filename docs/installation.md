# Installation

dygo release binaries are distributed through GitHub Releases.

The intended public install command is:

```sh
curl -fsSL https://dygo.dev/install | sh
```

Until `dygo.dev/install` is wired, use the repository-hosted installer:

```sh
curl -fsSL https://raw.githubusercontent.com/dygo-dev/dygo/main/scripts/install.sh | sh
```

The installer places the managed binary in:

```txt
~/.dygo/bin
```

If that directory is not on `PATH`, the installer prints the shell profile line to add.

## Options

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/dygo-dev/dygo/main/scripts/install.sh | DYGO_VERSION=v0.1.0 sh
```

Install somewhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/dygo-dev/dygo/main/scripts/install.sh | DYGO_INSTALL_DIR=/usr/local/bin sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/dygo-dev/dygo/main/scripts/install.ps1 | iex
```

## Upgrades

Upgrade the managed CLI:

```sh
dygo upgrade
```

Inside a generated dygo project, `dygo upgrade` also updates the project `go.mod` dygo dependency and dygo-managed generated runner files. Project upgrades refuse dirty git worktrees.

Useful upgrade modes:

```sh
dygo upgrade --check
dygo upgrade --dry-run
dygo upgrade --to v0.1.0
dygo upgrade --cli-only
dygo upgrade --project-only
```
