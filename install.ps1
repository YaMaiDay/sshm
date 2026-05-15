$ErrorActionPreference = "Stop"

$Repo = "YaMaiDay/sshm"
$Binary = "sshm.exe"
$InstallDir = if ($env:SSHM_INSTALL_DIR) { $env:SSHM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\sshm" }
$Version = if ($env:SSHM_VERSION) { $env:SSHM_VERSION } else { "latest" }

function Write-Info($Message) {
    Write-Host $Message
}

function Fail($Message) {
    Write-Error "Install failed: $Message"
    exit 1
}

function Detect-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Fail "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

if ($Version -eq "latest") {
    $response = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest" -MaximumRedirection 5
    $Version = Split-Path -Leaf $response.BaseResponse.ResponseUri.AbsoluteUri
}

if (-not $Version) {
    Fail "could not resolve latest version"
}

$Arch = Detect-Arch
$Asset = "sshm_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Asset"
$ChecksumsUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("sshm-install-" + [System.Guid]::NewGuid().ToString("N"))
$ZipPath = Join-Path $TempDir $Asset
$ChecksumsPath = Join-Path $TempDir "checksums.txt"

New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
    Write-Info "Downloading sshm $Version (windows/$Arch)..."
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsPath

    $ExpectedLine = Get-Content $ChecksumsPath | Where-Object { $_ -match "\s$([regex]::Escape($Asset))$" } | Select-Object -First 1
    if (-not $ExpectedLine) {
        Fail "$Asset was not found in checksums.txt"
    }
    $Expected = ($ExpectedLine -split "\s+")[0].Trim()
    $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected.ToLowerInvariant() -ne $Actual) {
        Fail "SHA256 mismatch: expected $Expected, got $Actual"
    }
    Write-Info "SHA256 verified"

    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

    $Source = Join-Path $TempDir $Binary
    if (-not (Test-Path $Source)) {
        Fail "$Binary was not found in the archive"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path $Source -Destination (Join-Path $InstallDir $Binary) -Force

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathParts = $UserPath -split ";"
    if ($PathParts -notcontains $InstallDir) {
        $NextPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable("Path", $NextPath, "User")
        Write-Info "Added to user PATH: $InstallDir"
        Write-Info "Reopen Windows Terminal, then run sshm."
    }

    Write-Info "Installed: $(Join-Path $InstallDir $Binary)"
    Write-Info "Run: sshm"
}
finally {
    if (Test-Path $TempDir) {
        Remove-Item -Recurse -Force $TempDir
    }
}
