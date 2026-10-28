# Installs the latest dgsl release on Windows.
#
#   irm https://raw.githubusercontent.com/te-o-o-o/dg-statuslines/main/install.ps1 | iex
#
# The binary goes to %LOCALAPPDATA%\dgsl, which is added to the user PATH. Running
# the script again updates dgsl in place, so the path Claude Code runs never changes.
$ErrorActionPreference = 'Stop'

$repo = 'te-o-o-o/dg-statuslines'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$file = "dgsl-windows-$arch.exe"
$base = "https://github.com/$repo/releases/latest/download"
$dir = Join-Path $env:LOCALAPPDATA 'dgsl'
New-Item -ItemType Directory -Force $dir | Out-Null

$tmp = Join-Path $dir 'dgsl.download'
$sums = Join-Path $dir 'checksums.txt'
Write-Host "Downloading $file..."
Invoke-WebRequest "$base/$file" -OutFile $tmp -UseBasicParsing
Invoke-WebRequest "$base/checksums.txt" -OutFile $sums -UseBasicParsing

# Refuse the binary unless it matches the checksum published with the release.
$line = Get-Content $sums | Where-Object { ($_ -split '\s+')[1] -eq $file }
$have = (Get-FileHash $tmp -Algorithm SHA256).Hash.ToLower()
Remove-Item $sums
if (-not $line -or ($line -split '\s+')[0] -ne $have) {
  Remove-Item $tmp
  throw 'dgsl: checksum mismatch, nothing installed.'
}

$exe = Join-Path $dir 'dgsl.exe'
Move-Item -Force $tmp $exe
Write-Host "Installed $(& $exe version) in $exe"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $dir) {
  [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
  Write-Host "Added $dir to your PATH. Open a new terminal, then run: dgsl"
} else {
  Write-Host 'Run: dgsl'
}
