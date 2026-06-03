#Requires -Version 5.1
<#
.SYNOPSIS
    TPT Healthcare NZ Windows Installer

.DESCRIPTION
    Installs tpt-health-interop on Windows, creates a Windows Service,
    writes configuration, and launches the first-run setup wizard.

.PARAMETER Uninstall
    Remove TPT Healthcare NZ from this machine.

.PARAMETER FromSource
    Build tpt-health-interop.exe from source (requires Go 1.22+).

.PARAMETER NoService
    Skip Windows Service registration.

.EXAMPLE
    .\install.ps1
    .\install.ps1 -FromSource
    .\install.ps1 -Uninstall
#>

[CmdletBinding(SupportsShouldProcess)]
param(
    [switch]$Uninstall,
    [switch]$FromSource,
    [switch]$NoService
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
$AppName        = 'tpt-health-interop'
$DisplayName    = 'TPT Health Interoperability Service'
$GithubOrg      = 'PhillipC05'
$GithubRepo     = 'tpt-healthcare-nz'
$ServiceName    = 'TptHealthInterop'
$ServiceUser    = 'NT AUTHORITY\LocalService'

# Paths
$InstallRoot    = 'C:\Program Files\TPT Healthcare'
$BinDir         = Join-Path $InstallRoot 'bin'
$DataDir        = Join-Path $InstallRoot 'data'
$ConfigDir      = Join-Path $env:APPDATA 'TPT Healthcare'
$ConfigFile     = Join-Path $ConfigDir   'config.yaml'
$EnvFile        = Join-Path $ConfigDir   'environment.env'
$LogDir         = Join-Path $env:LOCALAPPDATA 'TPT Healthcare\Logs'
$BinaryPath     = Join-Path $BinDir "$AppName.exe"

# Placeholder download URL — replace with the real release asset URL when
# GitHub releases are published.
# $DownloadUrlTemplate = "https://github.com/$GithubOrg/$GithubRepo/releases/download/{0}/tpt-health-interop_windows_amd64.zip"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Write-Info  ([string]$Msg) { Write-Host "[INFO]  $Msg" -ForegroundColor Cyan }
function Write-Ok    ([string]$Msg) { Write-Host "[OK]    $Msg" -ForegroundColor Green }
function Write-Warn  ([string]$Msg) { Write-Host "[WARN]  $Msg" -ForegroundColor Yellow }
function Write-Err   ([string]$Msg) { Write-Host "[ERROR] $Msg" -ForegroundColor Red }
function Write-Hdr   ([string]$Msg) { Write-Host "`n==> $Msg" -ForegroundColor Cyan -NoNewline; Write-Host '' }

function Fail([string]$Msg) {
    Write-Err $Msg
    exit 1
}

function Assert-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p  = New-Object Security.Principal.WindowsPrincipal($id)
    if (-not $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        Fail 'This installer must be run as Administrator. Right-click and select "Run as Administrator".'
    }
}

# ---------------------------------------------------------------------------
# PowerShell version check
# ---------------------------------------------------------------------------
function Test-PSVersion {
    Write-Hdr 'Checking PowerShell version'
    $ver = $PSVersionTable.PSVersion
    if ($ver.Major -lt 5 -or ($ver.Major -eq 5 -and $ver.Minor -lt 1)) {
        Fail "PowerShell 5.1 or later is required (found $ver). Download from https://aka.ms/wmf51"
    }
    Write-Ok "PowerShell $($ver.ToString())"
}

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
function Test-Prerequisites {
    Write-Hdr 'Checking prerequisites'

    $missing = @()

    # Docker Desktop
    $dockerCmd = Get-Command 'docker' -ErrorAction SilentlyContinue
    if ($dockerCmd) {
        try {
            $null = docker info 2>&1
            Write-Ok "Docker $(docker --version)"
        } catch {
            Write-Warn 'Docker is installed but the daemon is not running. Start Docker Desktop before running migrations.'
        }
    } else {
        $missing += 'Docker Desktop (https://www.docker.com/products/docker-desktop/)'
    }

    # Go 1.22+ (only for -FromSource)
    if ($FromSource) {
        $goCmd = Get-Command 'go' -ErrorAction SilentlyContinue
        if ($goCmd) {
            $goVerStr = (go version) -replace 'go version go', '' -replace ' .*', ''
            $parts    = $goVerStr -split '\.'
            $major    = [int]$parts[0]
            $minor    = [int]$parts[1]
            if ($major -lt 1 -or ($major -eq 1 -and $minor -lt 22)) {
                Fail "Go 1.22+ is required for -FromSource (found go$goVerStr). See https://go.dev/dl/"
            }
            Write-Ok "Go $goVerStr"
        } else {
            $missing += 'Go 1.22+ (https://go.dev/dl/) — required for -FromSource'
        }
    }

    if ($missing.Count -gt 0) {
        Write-Err 'Missing prerequisites:'
        foreach ($item in $missing) { Write-Err "  * $item" }
        Fail 'Please install the above and re-run the installer.'
    }
}

# ---------------------------------------------------------------------------
# Create directories
# ---------------------------------------------------------------------------
function New-Directories {
    Write-Hdr 'Creating directories'

    foreach ($dir in @($InstallRoot, $BinDir, $DataDir, $ConfigDir, $LogDir)) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            Write-Ok "Created $dir"
        } else {
            Write-Info "Exists: $dir"
        }
    }

    # Restrict config directory to Admins + SYSTEM only
    try {
        $acl   = Get-Acl $ConfigDir
        $acl.SetAccessRuleProtection($true, $false)
        $admins = [Security.Principal.NTAccount]'BUILTIN\Administrators'
        $system = [Security.Principal.NTAccount]'NT AUTHORITY\SYSTEM'
        foreach ($principal in @($admins, $system)) {
            $rule = New-Object Security.AccessControl.FileSystemAccessRule(
                $principal, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow'
            )
            $acl.AddAccessRule($rule)
        }
        Set-Acl -Path $ConfigDir -AclObject $acl
        Write-Ok "Config directory permissions restricted."
    } catch {
        Write-Warn "Could not restrict config directory ACL: $_"
    }
}

# ---------------------------------------------------------------------------
# Install binary
# ---------------------------------------------------------------------------
function Install-Binary {
    Write-Hdr "Installing $AppName"

    if ($FromSource) {
        Build-FromSource
        return
    }

    # Uncomment and adapt once releases are published to GitHub:
    #
    # Write-Info 'Fetching latest release tag...'
    # $releaseJson = Invoke-RestMethod `
    #     -Uri "https://api.github.com/repos/$GithubOrg/$GithubRepo/releases/latest" `
    #     -Headers @{ 'User-Agent' = 'tpt-installer' }
    # $tag = $releaseJson.tag_name
    # $url = $DownloadUrlTemplate -f $tag
    #
    # Write-Info "Downloading $url"
    # $zip = Join-Path $env:TEMP 'tpt-health-interop_windows_amd64.zip'
    # Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
    # Expand-Archive -Path $zip -DestinationPath $BinDir -Force
    # Remove-Item $zip -Force
    # Write-Ok "Downloaded and extracted to $BinDir"

    # If no release is available, fall back to source build automatically.
    Write-Warn 'Pre-built Windows binaries are not yet published.'
    Write-Warn 'Falling back to -FromSource build.'
    $script:FromSource = $true
    Build-FromSource
}

function Build-FromSource {
    Write-Info 'Building from source...'

    $scriptDir  = Split-Path -Parent $MyInvocation.ScriptName
    $sourceRoot = (Resolve-Path (Join-Path $scriptDir '..\..')).Path

    if (-not (Test-Path (Join-Path $sourceRoot 'go.work'))) {
        Fail "Source root not found at $sourceRoot. Cannot build from source."
    }

    $gitDesc = ''
    try { $gitDesc = git -C $sourceRoot describe --tags --always --dirty 2>$null } catch {}
    if (-not $gitDesc) { $gitDesc = 'dev' }

    $ldflags = "-s -w -X main.version=$gitDesc"
    $entryPkg = './interop/cmd/tpt-health-interop/...'

    Push-Location $sourceRoot
    try {
        & go build -ldflags $ldflags -o $BinaryPath $entryPkg
        if ($LASTEXITCODE -ne 0) { Fail 'go build failed.' }
    } finally {
        Pop-Location
    }

    Write-Ok "Built $BinaryPath"
}

# ---------------------------------------------------------------------------
# Generate encryption key
# ---------------------------------------------------------------------------
function New-EncryptionKey {
    $bytes = New-Object byte[] 32
    [Security.Cryptography.RNGCryptoServiceProvider]::Create().GetBytes($bytes)
    return ($bytes | ForEach-Object { '{0:x2}' -f $_ }) -join ''
}

# ---------------------------------------------------------------------------
# Write configuration
# ---------------------------------------------------------------------------
function Write-Config {
    Write-Hdr 'Writing configuration'

    $encKey   = New-EncryptionKey
    $timestamp = (Get-Date -Format 'o')

    # config.yaml
    $configYaml = @"
# TPT Healthcare NZ -- main configuration
# Generated by installer on $timestamp
# IMPORTANT: Keep this file private (restricted ACL, do not check into source control).

server:
  listen: "0.0.0.0:8080"
  tls_cert_file: ""
  tls_key_file: ""
  base_url: "http://localhost:8080"

database:
  # PostgreSQL DSN -- override via POSTGRES_DSN environment variable
  dsn: "postgres://tpt_healthcare:changeme@localhost:5432/tpt_healthcare?sslmode=prefer"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: "5m"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

auth:
  # mode: local_jwt | auth0 | tpt_identity
  mode: "local_jwt"
  jwt:
    private_key_file: "$($ConfigDir -replace '\\','/')/jwt_ed25519.pem"
    token_ttl: "1h"
    refresh_ttl: "24h"
  totp:
    issuer: "TPT Healthcare NZ"

logging:
  level: "info"
  format: "json"
  output: "$($LogDir -replace '\\','/')/interop.log"

migrations:
  auto_run: true

wizard:
  completed: false
"@

    Set-Content -Path $ConfigFile -Value $configYaml -Encoding UTF8
    Write-Ok "Config written to $ConfigFile"

    # environment.env
    $envContent = @"
# TPT Healthcare NZ -- environment file
# Generated by installer on $timestamp
# IMPORTANT: Contains secrets. Do not share or check into source control.

ENCRYPTION_KEY=$encKey
CONFIG_FILE=$ConfigFile
TPT_LOG_LEVEL=info
"@

    Set-Content -Path $EnvFile -Value $envContent -Encoding UTF8
    Write-Ok "Environment file written to $EnvFile"

    # Persist ENCRYPTION_KEY as a machine-level environment variable for the service
    [System.Environment]::SetEnvironmentVariable('ENCRYPTION_KEY', $encKey, 'Machine')
    [System.Environment]::SetEnvironmentVariable('TPT_CONFIG_FILE', $ConfigFile, 'Machine')
    Write-Ok 'Machine environment variables set (ENCRYPTION_KEY, TPT_CONFIG_FILE).'
}

# ---------------------------------------------------------------------------
# Database migrations
# ---------------------------------------------------------------------------
function Invoke-Migrations {
    Write-Hdr 'Running database migrations'

    if (-not (Test-Path $BinaryPath)) {
        Write-Warn "Binary not found at $BinaryPath -- skipping automatic migrations."
        Write-Warn "Run manually: $BinaryPath migrate --config `"$ConfigFile`""
        return
    }

    try {
        & $BinaryPath migrate --config $ConfigFile
        if ($LASTEXITCODE -ne 0) { throw "exit code $LASTEXITCODE" }
        Write-Ok 'Migrations applied.'
    } catch {
        Write-Warn "Automatic migration failed: $_"
        Write-Warn "Run manually: $BinaryPath migrate --config `"$ConfigFile`""
    }
}

# ---------------------------------------------------------------------------
# Windows Service
# ---------------------------------------------------------------------------
function Install-WindowsService {
    if ($NoService) {
        Write-Info 'Skipping Windows Service installation (-NoService).'
        return
    }

    Write-Hdr 'Registering Windows Service'

    # Remove stale service if present
    $existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Info "Service '$ServiceName' already exists; removing to re-register."
        if ($existing.Status -eq 'Running') {
            Stop-Service -Name $ServiceName -Force
        }
        & sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }

    $binPathWithArgs = "`"$BinaryPath`" serve --config `"$ConfigFile`""

    New-Service `
        -Name        $ServiceName `
        -DisplayName $DisplayName `
        -Description 'TPT Healthcare NZ FHIR interoperability engine (NHI, NES, ACC, HPI).' `
        -BinaryPathName $binPathWithArgs `
        -StartupType Automatic | Out-Null

    # Set service to run as LocalService and depend on key services
    & sc.exe config $ServiceName obj= $ServiceUser | Out-Null
    & sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    Start-Service -Name $ServiceName
    Write-Ok "Windows Service '$ServiceName' installed and started."
    Write-Info "Manage with: Get-Service $ServiceName | Start-Service / Stop-Service"
}

# ---------------------------------------------------------------------------
# Start Menu shortcut
# ---------------------------------------------------------------------------
function New-StartMenuShortcut {
    Write-Hdr 'Creating Start Menu shortcut'

    $startMenuDir = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\TPT Healthcare'
    if (-not (Test-Path $startMenuDir)) {
        New-Item -ItemType Directory -Path $startMenuDir -Force | Out-Null
    }

    $shell    = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut((Join-Path $startMenuDir 'TPT Healthcare Setup Wizard.lnk'))
    $shortcut.TargetPath       = 'http://localhost:8080/setup'
    $shortcut.Description      = 'Open TPT Healthcare first-run setup wizard'
    $shortcut.IconLocation     = "$BinaryPath,0"
    $shortcut.Save()

    Write-Ok "Start Menu shortcut created in $startMenuDir"
}

# ---------------------------------------------------------------------------
# Open first-run wizard in browser
# ---------------------------------------------------------------------------
function Open-Wizard {
    Write-Info 'Opening first-run wizard in default browser...'
    Start-Sleep -Seconds 3
    try {
        Start-Process 'http://localhost:8080/setup'
    } catch {
        Write-Warn 'Could not open browser automatically. Navigate to http://localhost:8080/setup'
    }
}

# ---------------------------------------------------------------------------
# Uninstall
# ---------------------------------------------------------------------------
function Invoke-Uninstall {
    Write-Hdr 'Uninstalling TPT Healthcare NZ'
    Assert-Admin

    # Stop and remove service
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($svc) {
        if ($svc.Status -eq 'Running') { Stop-Service -Name $ServiceName -Force }
        & sc.exe delete $ServiceName | Out-Null
        Write-Ok "Windows Service '$ServiceName' removed."
    }

    # Remove files
    foreach ($dir in @($InstallRoot, $DataDir)) {
        if (Test-Path $dir) {
            Remove-Item $dir -Recurse -Force
            Write-Ok "Removed $dir"
        }
    }

    # Remove machine env vars
    [System.Environment]::SetEnvironmentVariable('ENCRYPTION_KEY',  $null, 'Machine')
    [System.Environment]::SetEnvironmentVariable('TPT_CONFIG_FILE', $null, 'Machine')
    Write-Ok 'Machine environment variables cleared.'

    # Remove Start Menu shortcut
    $startMenuDir = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\TPT Healthcare'
    if (Test-Path $startMenuDir) {
        Remove-Item $startMenuDir -Recurse -Force
        Write-Ok 'Start Menu shortcuts removed.'
    }

    Write-Warn "Config directory $ConfigDir NOT removed (may contain secrets)."
    Write-Warn "Remove manually: Remove-Item '$ConfigDir' -Recurse -Force"

    Write-Ok 'Uninstall complete.'
    exit 0
}

# ---------------------------------------------------------------------------
# Success message
# ---------------------------------------------------------------------------
function Write-Success {
    Write-Host ''
    Write-Host '=================================================' -ForegroundColor Green
    Write-Host '  TPT Healthcare NZ installed successfully!'       -ForegroundColor Green
    Write-Host '=================================================' -ForegroundColor Green
    Write-Host ''
    Write-Host "  Binary   : $BinaryPath"
    Write-Host "  Config   : $ConfigFile"
    Write-Host "  Env file : $EnvFile"
    Write-Host "  Logs     : $LogDir"
    Write-Host ''
    Write-Host 'Next steps:' -ForegroundColor Cyan
    Write-Host "  1. Edit $ConfigFile to set your PostgreSQL DSN."
    Write-Host "  2. Edit $EnvFile to review your ENCRYPTION_KEY."
    Write-Host '  3. Open the first-run wizard: http://localhost:8080/setup'
    Write-Host ''
    Write-Host 'Security reminder:' -ForegroundColor Yellow
    Write-Host '  * Change the default database password immediately.'
    Write-Host '  * Store your ENCRYPTION_KEY in a secret manager.'
    Write-Host '  * Configure TLS before accepting any patient data.'
    Write-Host '  * This system stores health information governed by the'
    Write-Host '    NZ Privacy Act 2020 and the Health Information Privacy'
    Write-Host '    Code 2020.'
    Write-Host ''
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
Assert-Admin
Test-PSVersion

if ($Uninstall) {
    Invoke-Uninstall
}

Test-Prerequisites
New-Directories
Install-Binary
Write-Config
Invoke-Migrations
Install-WindowsService
New-StartMenuShortcut
Write-Success
Open-Wizard
