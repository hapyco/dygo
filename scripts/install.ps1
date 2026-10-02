$ErrorActionPreference = "Stop"

$Repo = "hapyco/dygo"
$Version = if ($env:DYGO_VERSION) { $env:DYGO_VERSION } else { "latest" }
$InstallDir = if ($env:DYGO_INSTALL_DIR) { $env:DYGO_INSTALL_DIR } else { Join-Path $HOME ".dygo\bin" }
function Write-InstallProgress([string] $Message) {
  [Console]::Error.WriteLine("$Message...")
}

$DownloadBaseURL = if ($env:DYGO_DOWNLOAD_BASE_URL) { $env:DYGO_DOWNLOAD_BASE_URL } else { $null }

$ArchName = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()
switch ($ArchName) {
  "x64" { $GoArch = "amd64" }
  "arm64" { $GoArch = "arm64" }
  default { throw "unsupported architecture: $ArchName" }
}

if ($Version -eq "latest") {
  Write-InstallProgress "Resolving latest dygo release"
  $Headers = @{ Accept = "application/vnd.github+json"; "User-Agent" = "dygo-installer" }
  $Latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers $Headers
  $Version = $Latest.tag_name
}
if (-not $Version) {
  throw "could not resolve dygo version"
}
if (-not $Version.StartsWith("v")) {
  $Version = "v$Version"
}
if ($Version -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$') {
  throw "invalid dygo version: $Version"
}

$Asset = "dygo_${Version}_windows_${GoArch}.zip"
$BaseURL = if ($DownloadBaseURL) { $DownloadBaseURL } else { "https://github.com/$Repo/releases/download/$Version" }
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("dygo-install-" + [System.Guid]::NewGuid())
$StagedBinary = $null
$BackupBinary = $null
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
  $ArchivePath = Join-Path $TempDir $Asset
  $ChecksumsPath = Join-Path $TempDir "checksums.txt"
  Write-InstallProgress "Downloading dygo $Version"
  Invoke-WebRequest -UseBasicParsing -Uri "$BaseURL/$Asset" -OutFile $ArchivePath
  Invoke-WebRequest -UseBasicParsing -Uri "$BaseURL/checksums.txt" -OutFile $ChecksumsPath

  Write-InstallProgress "Installing dygo $Version"
  $Expected = (Get-Content $ChecksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($Asset))$" } | ForEach-Object { ($_ -split "\s+")[0] } | Select-Object -First 1)
  if (-not $Expected) {
    throw "checksums.txt does not contain $Asset"
  }
  $Actual = (Get-FileHash -Algorithm SHA256 $ArchivePath).Hash.ToLowerInvariant()
  if ($Actual -ne $Expected.ToLowerInvariant()) {
    throw "checksum mismatch for $Asset"
  }

  Expand-Archive -Path $ArchivePath -DestinationPath $TempDir -Force
  $ExtractedBinary = Join-Path $TempDir "dygo.exe"
  if (-not (Test-Path -Path $ExtractedBinary -PathType Leaf)) {
    throw "release archive does not contain dygo.exe"
  }
  $ReportedVersion = (& $ExtractedBinary version | Out-String).Trim()
  if ($LASTEXITCODE -ne 0) {
    throw "downloaded binary could not report its version"
  }
  if ($ReportedVersion -ne "dygo $Version") {
    throw "downloaded binary version does not match $Version"
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $BinaryPath = Join-Path $InstallDir "dygo.exe"
  $StagedBinary = Join-Path $InstallDir (".dygo-install-" + [System.Guid]::NewGuid() + ".exe")
  Copy-Item -Path $ExtractedBinary -Destination $StagedBinary -Force
  if (Test-Path -Path $BinaryPath -PathType Leaf) {
    $BackupBinary = Join-Path $InstallDir (".dygo-backup-" + [System.Guid]::NewGuid() + ".exe")
    [System.IO.File]::Replace($StagedBinary, $BinaryPath, $BackupBinary, $true)
    Remove-Item -Path $BackupBinary -Force
    $BackupBinary = $null
  }
  else {
    [System.IO.File]::Move($StagedBinary, $BinaryPath)
  }
  $StagedBinary = $null

  Write-Host "dygo $Version installed to $BinaryPath"
  if (($env:PATH -split ";") -notcontains $InstallDir) {
    Write-Host "Add $InstallDir to your PATH."
  }
}
finally {
  Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
  if ($StagedBinary) {
    Remove-Item -Path $StagedBinary -Force -ErrorAction SilentlyContinue
  }
  if ($BackupBinary) {
    Remove-Item -Path $BackupBinary -Force -ErrorAction SilentlyContinue
  }
}
