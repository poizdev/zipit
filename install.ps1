$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Fail([string] $Message) {
    throw "zipit installer: $Message"
}

function Test-PathContains([string[]] $Entries, [string] $Directory) {
    $normalizedDirectory = $Directory.TrimEnd([char[]] @('\', '/'))
    foreach ($entry in $Entries) {
        $expandedEntry = [Environment]::ExpandEnvironmentVariables($entry.Trim())
        if ($expandedEntry.TrimEnd([char[]] @('\', '/')).Equals(
            $normalizedDirectory,
            [System.StringComparison]::OrdinalIgnoreCase
        )) {
            return $true
        }
    }
    return $false
}

if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
    [System.Runtime.InteropServices.OSPlatform]::Windows
)) {
    Fail "this installer supports Windows only"
}

$repository = if ($env:ZIPIT_GITHUB_REPOSITORY) {
    $env:ZIPIT_GITHUB_REPOSITORY
} else {
    "poizdev/zipit"
}

if ($repository -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
    Fail "ZIPIT_GITHUB_REPOSITORY must be in owner/repository form"
}

$version = $env:ZIPIT_VERSION
if (-not $version) {
    $releaseUrl = "https://api.github.com/repos/$repository/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{ Accept = "application/vnd.github+json" }
    } catch {
        Fail "could not find the latest stable release: $($_.Exception.Message)"
    }
    if ($release.draft -or $release.prerelease) {
        Fail "GitHub returned a draft or prerelease as the latest stable release"
    }
    $version = [string] $release.tag_name
}

if ($version -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
    Fail "version must be a stable vMAJOR.MINOR.PATCH tag: $version"
}
$artifactVersion = $version.Substring(1)

$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
switch ($architecture) {
    "X64" { $arch = "amd64" }
    "Arm64" { $arch = "arm64" }
    default { Fail "unsupported Windows architecture: $architecture" }
}

$asset = "zipit_${artifactVersion}_windows_${arch}.zip"
$baseUrl = "https://github.com/$repository/releases/download/$version"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("zipit-install-" + [guid]::NewGuid().ToString("N"))
$archive = Join-Path $tempDir $asset
$checksums = Join-Path $tempDir "checksums.txt"
$extractedBinary = Join-Path $tempDir "zipit.exe"
$stage = $null
$backup = $null

try {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    Write-Host "Installing Zipit $version for windows/$arch..."

    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$asset" -OutFile $archive
        Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/checksums.txt" -OutFile $checksums
    } catch {
        Fail "release download failed: $($_.Exception.Message)"
    }

    $checksumText = [System.IO.File]::ReadAllText($checksums)
    $assetPattern = [regex]::Escape($asset)
    $checksumMatches = [regex]::Matches(
        $checksumText,
        "(?m)^([0-9A-Fa-f]{64})\s+\*?$assetPattern\r?$"
    )
    if ($checksumMatches.Count -ne 1) {
        Fail "checksums.txt must contain exactly one SHA-256 entry for $asset"
    }
    $expectedHash = $checksumMatches[0].Groups[1].Value
    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $archive).Hash
    if (-not $actualHash.Equals($expectedHash, [System.StringComparison]::OrdinalIgnoreCase)) {
        Fail "checksum verification failed for $asset"
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($archive)
    try {
        if ($zip.Entries.Count -ne 1 -or $zip.Entries[0].FullName -ne "zipit.exe") {
            Fail "$asset contains unexpected files"
        }
        $inputStream = $zip.Entries[0].Open()
        $outputStream = [System.IO.File]::Create($extractedBinary)
        try {
            $inputStream.CopyTo($outputStream)
        } finally {
            $outputStream.Dispose()
            $inputStream.Dispose()
        }
    } finally {
        $zip.Dispose()
    }

    if (-not $env:LOCALAPPDATA) {
        Fail "LOCALAPPDATA is not set"
    }
    $installDir = Join-Path $env:LOCALAPPDATA "Programs\Zipit\bin"
    $installPath = Join-Path $installDir "zipit.exe"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null

    $stage = Join-Path $installDir (".zipit.tmp." + [guid]::NewGuid().ToString("N"))
    [System.IO.File]::Copy($extractedBinary, $stage, $true)
    if (Test-Path -LiteralPath $installPath) {
        $backup = Join-Path $installDir (".zipit.backup." + [guid]::NewGuid().ToString("N"))
        [System.IO.File]::Replace($stage, $installPath, $backup, $true)
        Remove-Item -LiteralPath $backup -Force
        $backup = $null
    } else {
        [System.IO.File]::Move($stage, $installPath)
    }
    $stage = $null

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $userEntries = @($userPath -split ';' | Where-Object { $_ })
    $userHasPath = Test-PathContains $userEntries $installDir
    if (-not $userHasPath) {
        $newUserPath = if ($userPath) { "$($userPath.TrimEnd(';'));$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    }

    $processEntries = @($env:Path -split ';' | Where-Object { $_ })
    $processHasPath = Test-PathContains $processEntries $installDir
    if (-not $processHasPath) {
        $env:Path = "$installDir;$env:Path"
    }

    Write-Host "Installed Zipit to $installPath"
    if (-not $userHasPath) {
        Write-Host "User PATH updated. New terminal sessions can run zipit."
    }
} finally {
    if ($stage -and (Test-Path -LiteralPath $stage)) {
        Remove-Item -LiteralPath $stage -Force
    }
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}
