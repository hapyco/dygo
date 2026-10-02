$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
$TestRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("dygo-installer-test-" + [System.Guid]::NewGuid())
$ReleaseDir = Join-Path $TestRoot "release"
$InstallDir = Join-Path $TestRoot "install with spaces"
$Version = "v0.0.0-ci"
$PreviousVersion = "v0.0.6"
$GoArch = switch ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString()) {
  "X64" { "amd64" }
  "Arm64" { "arm64" }
  default { throw "unsupported test architecture" }
}
$Asset = "dygo_${Version}_windows_${GoArch}.zip"
$BinaryPath = Join-Path $InstallDir "dygo.exe"
$Installer = Join-Path $PSScriptRoot "install.ps1"
$Server = $null
$OldEnvironment = @{}
foreach ($Name in @("DYGO_VERSION", "DYGO_INSTALL_DIR", "DYGO_DOWNLOAD_BASE_URL")) {
  $OldEnvironment[$Name] = [Environment]::GetEnvironmentVariable($Name)
}

function Pack-Release([string] $Binary) {
  Compress-Archive -Path $Binary -DestinationPath (Join-Path $ReleaseDir $Asset) -Force
  $Hash = (Get-FileHash -Algorithm SHA256 (Join-Path $ReleaseDir $Asset)).Hash.ToLowerInvariant()
  "$Hash  $Asset" | Set-Content -Path (Join-Path $ReleaseDir "checksums.txt") -Encoding ascii
}

function Assert-Version([string] $Expected) {
  $Reported = & $BinaryPath version
  if ($LASTEXITCODE -ne 0 -or $Reported -ne "dygo $Expected") {
    throw "installed Windows binary version is incorrect: $Reported"
  }
}

function Assert-Preserved {
  Assert-Version $Version
  if ((Get-FileHash $BinaryPath).Hash -ne $script:OriginalHash) {
    throw "failed update changed the installed binary"
  }
  if (Get-ChildItem -Path $InstallDir -Filter ".dygo-*" -Force) {
    throw "installer left staging or backup files behind"
  }
}

function Expect-Failure([string] $Diagnostic) {
  $Rejected = $false
  try {
    & $Installer
  }
  catch {
    if ($_.Exception.Message -notmatch $Diagnostic) { throw }
    $Rejected = $true
  }
  if (-not $Rejected) { throw "PowerShell installer accepted invalid installation: $Diagnostic" }
  Assert-Preserved
}

try {
  New-Item -ItemType Directory -Path $ReleaseDir -Force | Out-Null
  $PayloadDir = Join-Path $TestRoot "candidate"
  New-Item -ItemType Directory -Path $PayloadDir | Out-Null
  $Candidate = Join-Path $PayloadDir "dygo.exe"
  Push-Location $RepoRoot
  try {
    go build -trimpath -ldflags "-s -w -X github.com/hapyco/dygo/internal/cli.version=$Version" -o $Candidate ./cmd/dygo
    if ($LASTEXITCODE -ne 0) { throw "candidate build failed" }
  }
  finally { Pop-Location }
  Pack-Release $Candidate

  $Listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
  $Listener.Start()
  $Port = ([System.Net.IPEndPoint]$Listener.LocalEndpoint).Port
  $Listener.Stop()
  $Server = Start-Process python -ArgumentList @("-m", "http.server", "$Port", "--bind", "127.0.0.1", "--directory", "`"$ReleaseDir`"") -PassThru
  $LocalURL = "http://127.0.0.1:$Port"
  for ($Attempt = 0; $Attempt -lt 20; $Attempt++) {
    try {
      Invoke-WebRequest -UseBasicParsing -Uri "$LocalURL/checksums.txt" -OutFile (Join-Path $TestRoot "probe.txt")
      break
    }
    catch {
      if ($Attempt -eq 19) { throw }
      Start-Sleep -Milliseconds 250
    }
  }

  # Install an actual released binary, then exercise replacement with the candidate.
  $env:DYGO_INSTALL_DIR = $InstallDir
  $env:DYGO_VERSION = $PreviousVersion
  Remove-Item Env:DYGO_DOWNLOAD_BASE_URL -ErrorAction SilentlyContinue
  & $Installer
  Assert-Version $PreviousVersion
  $env:DYGO_VERSION = $Version
  $env:DYGO_DOWNLOAD_BASE_URL = $LocalURL
  & $Installer
  Assert-Version $Version

  # Explicit downgrade uses the same verified, atomic path.
  $env:DYGO_VERSION = $PreviousVersion
  Remove-Item Env:DYGO_DOWNLOAD_BASE_URL
  & $Installer
  Assert-Version $PreviousVersion
  $env:DYGO_VERSION = $Version
  $env:DYGO_DOWNLOAD_BASE_URL = $LocalURL
  & $Installer
  Assert-Version $Version
  $script:OriginalHash = (Get-FileHash $BinaryPath).Hash

  $env:DYGO_VERSION = "not-a-version"
  Expect-Failure "invalid dygo version"
  $env:DYGO_VERSION = $Version
  Add-Content -Path (Join-Path $ReleaseDir $Asset) -Value "corrupt" -NoNewline
  Expect-Failure "checksum mismatch"
  Pack-Release $Candidate

  $env:DYGO_DOWNLOAD_BASE_URL = "$LocalURL/missing"
  Expect-Failure "404"
  $env:DYGO_DOWNLOAD_BASE_URL = $LocalURL

  # Use a native executable to test both incorrect output and nonzero exit status.
  $FixtureDir = Join-Path $TestRoot "fixture"
  New-Item -ItemType Directory -Path $FixtureDir | Out-Null
  $FixtureSource = Join-Path $FixtureDir "main.go"
  $FixtureBinary = Join-Path $FixtureDir "dygo.exe"
  @'
package main
import ("fmt"; "os")
var version string
var fail string
func main() { fmt.Println("dygo " + version); if fail == "yes" { os.Exit(42) } }
'@ | Set-Content -Path $FixtureSource -Encoding ascii
  foreach ($Failure in @("mismatch", "nonzero")) {
    $FixtureVersion = if ($Failure -eq "mismatch") { $PreviousVersion } else { $Version }
    $Fail = if ($Failure -eq "nonzero") { "yes" } else { "no" }
    go build -ldflags "-X main.version=$FixtureVersion -X main.fail=$Fail" -o $FixtureBinary $FixtureSource
    if ($LASTEXITCODE -ne 0) { throw "fixture build failed" }
    Pack-Release $FixtureBinary
    $Diagnostic = if ($Failure -eq "mismatch") { "version does not match" } else { "could not report its version" }
    Expect-Failure $Diagnostic
  }
  Pack-Release $Candidate

  # Windows denies replacement when another process holds a handle without delete sharing.
  $Locked = [System.IO.File]::Open($BinaryPath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::Read)
  try { Expect-Failure "used by another process|being used|access.*denied" }
  finally { $Locked.Dispose() }
  Assert-Preserved

  Write-Host "Windows installer lifecycle passed: released $PreviousVersion, upgrade, downgrade, verification and locked-binary recovery"
}
finally {
  foreach ($Name in $OldEnvironment.Keys) {
    [Environment]::SetEnvironmentVariable($Name, $OldEnvironment[$Name])
  }
  if ($Server) { Stop-Process -Id $Server.Id -Force -ErrorAction SilentlyContinue }
  Remove-Item -Path $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
