# SpoTUI installer for Windows 11
#
# Installs Go (and librespot for local audio) if missing, builds the binary,
# and places it on your PATH so you can launch it by typing `spotui`.
#
# Usage (from inside the cloned repo, in PowerShell):
#   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
#   .\install.ps1
#
# Flags:
#   -SkipLibrespot   Don't install librespot (control-only mode, no PC audio).
#
# Use Windows Terminal for proper TUI rendering.

param(
    [switch]$SkipLibrespot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# -- Output helpers ----------------------------------------------------------
function Write-Info { param($msg) Write-Host "==> " -ForegroundColor Green  -NoNewline; Write-Host $msg }
function Write-Warn { param($msg) Write-Host "!!  " -ForegroundColor Yellow -NoNewline; Write-Host $msg }
function Write-Err  { param($msg) Write-Host "xx  " -ForegroundColor Red    -NoNewline; Write-Host $msg }
function Test-Have  { param($cmd) $null -ne (Get-Command $cmd -ErrorAction SilentlyContinue) }

function Update-Path {
    # Refresh this session's PATH from the machine + user environment so freshly
    # installed tools become visible without opening a new terminal.
    $machine = [System.Environment]::GetEnvironmentVariable("PATH", "Machine")
    $user    = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    $env:PATH = "$machine;$user;$env:USERPROFILE\.cargo\bin"
}

# -- Must run from the repo root ---------------------------------------------
if (-not (Test-Path "go.mod") -or -not (Test-Path "cmd\spotui")) {
    Write-Err "Run this script from the SpoTUI repo root (where go.mod lives)."
    exit 1
}

# -- Detect package manager --------------------------------------------------
$PM = ""
if     (Test-Have "winget") { $PM = "winget" }
elseif (Test-Have "scoop")  { $PM = "scoop" }
elseif (Test-Have "choco")  { $PM = "choco" }

if ($PM -eq "") {
    Write-Warn "No supported package manager found (winget, scoop, or choco)."
    Write-Warn "Install Go manually from https://go.dev/dl, then re-run this script."
    $PM = "none"
}

# -- Ensure Go ---------------------------------------------------------------
if (Test-Have "go") {
    $goVer = (go version) -replace "go version ", ""
    Write-Info "Go already installed ($goVer)."
} else {
    Write-Info "Installing Go..."
    switch ($PM) {
        "winget" { winget install --id GoLang.Go -e --accept-source-agreements --accept-package-agreements }
        "scoop"  { scoop install go }
        "choco"  { choco install golang -y }
        "none"   { Write-Err "Cannot install Go automatically. Get it from https://go.dev/dl and re-run."; exit 1 }
    }
    Update-Path
    if (-not (Test-Have "go")) {
        Write-Err "Go still not on PATH after install. Open a new terminal, confirm 'go version' works, then re-run."
        exit 1
    }
}

# -- Ensure librespot (local audio playback) ---------------------------------
# Windows has no spotifyd build, so SpoTUI uses librespot instead — the same
# engine spotifyd is built on. It registers a Spotify Connect device that the
# app then controls, giving you local PC playback just like on Linux.
if ($SkipLibrespot) {
    Write-Warn "Skipping librespot install (-SkipLibrespot). SpoTUI will run in control-only mode."
} elseif (Test-Have "librespot") {
    Write-Info "librespot already installed."
} else {
    Write-Info "Installing librespot (for local PC playback)..."
    $installed = $false

    # Prefer scoop if available — fast, no compilation.
    if ($PM -eq "scoop") {
        try { scoop install librespot; $installed = Test-Have "librespot" } catch { $installed = $false }
    }

    # Fall back to building from source with cargo (Rust). No prebuilt Windows
    # binaries are published, so this compiles librespot — it can take several
    # minutes the first time.
    if (-not $installed) {
        if (-not (Test-Have "cargo")) {
            Write-Info "Installing the Rust toolchain (needed to build librespot)..."
            switch ($PM) {
                "winget" { winget install --id Rustlang.Rustup -e --accept-source-agreements --accept-package-agreements }
                "scoop"  { scoop install rustup; rustup default stable }
                "choco"  { choco install rustup.install -y }
                default  { Write-Warn "Install Rust from https://rustup.rs, then re-run." }
            }
            Update-Path
        }

        if (Test-Have "cargo") {
            Write-Info "Compiling librespot with cargo (this may take a few minutes)..."
            try { cargo install librespot; Update-Path; $installed = Test-Have "librespot" } catch { $installed = $false }
        }
    }

    if (-not $installed) {
        Write-Warn "Couldn't install librespot automatically."
        Write-Warn "SpoTUI will still build and run, but without local PC playback."
        Write-Warn "Install it later with:  cargo install librespot   (needs Rust from https://rustup.rs)"
    }
}

# -- Build -------------------------------------------------------------------
Write-Info "Building spotui.exe..."
go build -o spotui.exe .\cmd\spotui
if (-not (Test-Path "spotui.exe")) {
    Write-Err "Build failed - spotui.exe was not produced."
    exit 1
}

# -- Install on PATH ---------------------------------------------------------
$installDir = "$env:LOCALAPPDATA\Programs\spotui"
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir | Out-Null
}
Copy-Item -Force "spotui.exe" "$installDir\spotui.exe"
Remove-Item "spotui.exe"
Write-Info "Installed to $installDir\spotui.exe"

$userPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$installDir*") {
    [System.Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
    Write-Info "Added $installDir to your user PATH."
    Write-Warn "Open a new terminal for the PATH change to take effect."
} else {
    Write-Info "$installDir is already on your PATH."
}

Write-Host ""
Write-Info "Done! Open a new terminal and run:  spotui"
Write-Host ""
Write-Host "  Requirements:"
Write-Host "   * Spotify Premium account"
Write-Host "   * First run opens your browser to log in (control + librespot)"
Write-Host "   * Use Windows Terminal for the best TUI rendering"
if (-not $SkipLibrespot -and -not (Test-Have "librespot")) {
    Write-Warn "librespot is not installed yet - install it for local PC playback."
}
