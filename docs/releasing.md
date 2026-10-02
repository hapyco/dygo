# Release Process

dygo publishes versioned binaries through GitHub Releases. An annotated semantic-version tag starts the release workflow.

## Prepare And Push A Release

Start from a clean `main` branch that matches `origin/main`:

```sh
scripts/release.sh v0.1.0
```

This command:

1. Verifies the branch, worktree, remote state, and tag.
2. Tests and builds Studio.
3. Verifies the bundled Core App.
4. Runs Go tests and vet.
5. Tests the POSIX installer lifecycle, including replacement and failure recovery.
6. Builds all release archives and checksums.
7. Smokes the archive for the current operating system and architecture.
8. Creates an annotated local tag.

Review `dist/`, then push the tag:

```sh
git push origin refs/tags/v0.1.0
```

If you find a problem before push, delete the local tag with `git tag -d v0.1.0`, fix the problem, and run the release command again.

To push the tag after all checks pass, use:

```sh
scripts/release.sh v0.1.0 --push
```

Do not create release tags from feature branches or dirty worktrees.

## GitHub Release Workflow

The tag starts `.github/workflows/release.yml`. The workflow validates that the tag:

- uses `vMAJOR.MINOR.PATCH` or a valid prerelease suffix;
- is an annotated tag;
- points to a commit contained in `main`.

The workflow rebuilds and tests the framework, creates the release artifacts, generates provenance attestations, and publishes GitHub-generated release notes. A tag with a suffix such as `v0.2.0-rc.1` creates a prerelease.

## Release Artifacts

Each release contains:

- macOS binaries for AMD64 and ARM64;
- Linux binaries for AMD64 and ARM64;
- Windows binaries for AMD64 and ARM64;
- `checksums.txt`;
- `install.sh` and `install.ps1`;
- the repository license.

`scripts/build-release.sh <version> [output-dir]` builds the same artifacts without creating a tag. `scripts/smoke-release.sh <version> [output-dir]` verifies the archive for the current host.

## Installation Verification

The installers select the archive for the host, verify its SHA-256 checksum, extract the binary, and require `dygo version` to exit successfully and match the requested release. They then replace the managed binary atomically. A failed download, verification, or replacement must preserve an existing installation.

Use the release-hosted installers documented in [Installation](installation.md). Do not use a moving source-branch installer for a versioned production installation.

Pull-request CI runs the shell installer lifecycle tests on Linux and macOS. Windows runs `scripts/test-install.ps1` with PowerShell 7 and Windows PowerShell 5.1. The native macOS and Windows jobs install the actual v0.0.6 release and replace it with a binary built from the candidate commit. Windows also tests downgrade and recovery when the destination executable is locked. Shell regressions cover downgrade, corrupted or missing downloads, incorrect or failing version probes, failed replacement, and INT/TERM cancellation.

Linux CI and the release workflow provision PostgreSQL 17 for database regressions. For local release checks, set `DYGO_TEST_DATABASE_URL` to a disposable PostgreSQL service whose user can create databases. Tests create and remove isolated databases; without this variable, PostgreSQL tests are skipped and that limitation must be reported.

Before publishing, verify those native CI jobs for the exact release commit. A Linux build of a Windows or macOS archive does not prove installation on those hosts. After publication, download the release-hosted installer and assets, verify checksums, install the release in an isolated directory, and verify its reported version. Also test replacement of an actual prior release and run a generated-project smoke check. Record which operating systems and architectures were exercised and disclose any untested paths.
