$ErrorActionPreference = "Stop"

$Repo = "hapyco/dygo"
$Version = if ($env:DYGO_VERSION) { $env:DYGO_VERSION } else { "latest" }
$InstallDir = if ($env:DYGO_INSTALL_DIR) { $env:DYGO_INSTALL_DIR } else { Join-Path $HOME ".dygo\bin" }

$ArchName = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()
switch ($ArchName) {
  "x64" { $GoArch = "amd64" }
  "arm64" { $GoArch = "arm64" }
  default { throw "unsupported architecture: $ArchName" }
}

if ($Version -eq "latest") {
  $Latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $Latest.tag_name
}
if (-not $Version) {
  throw "could not resolve dygo version"
}

$Asset = "dygo_${Version}_windows_${GoArch}.zip"
$BaseURL = "https://github.com/$Repo/releases/download/$Version"
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("dygo-install-" + [System.Guid]::NewGuid())
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
  $ArchivePath = Join-Path $TempDir $Asset
  $ChecksumsPath = Join-Path $TempDir "checksums.txt"
  Invoke-WebRequest -Uri "$BaseURL/$Asset" -OutFile $ArchivePath
  Invoke-WebRequest -Uri "$BaseURL/checksums.txt" -OutFile $ChecksumsPath

  $Expected = (Get-Content $ChecksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($Asset))$" } | ForEach-Object { ($_ -split "\s+")[0] } | Select-Object -First 1)
  if (-not $Expected) {
    throw "checksums.txt does not contain $Asset"
  }
  $Actual = (Get-FileHash -Algorithm SHA256 $ArchivePath).Hash.ToLowerInvariant()
  if ($Actual -ne $Expected.ToLowerInvariant()) {
    throw "checksum mismatch for $Asset"
  }

  Expand-Archive -Path $ArchivePath -DestinationPath $TempDir -Force
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $BinaryPath = Join-Path $InstallDir "dygo.exe"
  Copy-Item -Path (Join-Path $TempDir "dygo.exe") -Destination $BinaryPath -Force

  Write-Host "dygo $Version installed to $BinaryPath"
  if (($env:PATH -split ";") -notcontains $InstallDir) {
    Write-Host "Add $InstallDir to your PATH."
  }
}
finally {
  Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
