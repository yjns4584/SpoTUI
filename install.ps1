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
#
# No prebuilt librespot binary is published for Windows (it isn't in scoop or as
# a GitHub release asset), so it's compiled from source with cargo. That needs a
# C linker/assembler:
#   * the MSVC linker (link.exe), from Visual Studio Build Tools, or
#   * a full MinGW-w64 toolchain used with Rust's GNU target.
# rustup's bundled "self-contained" MinGW is incomplete (it has dlltool/ld/gcc
# but no as.exe), so when going the GNU route we install a real MinGW-w64. The
# GNU route needs no admin rights, so we prefer it whenever link.exe is absent.

# Ensure-MinGW puts a complete MinGW-w64 (as.exe + dlltool.exe) on PATH, required
# to build the windows-* crates with Rust's GNU target. scoop's 'gcc' and choco's
# 'mingw' both ship full binutils; winget has no good mingw package, so we
# bootstrap scoop (no admin) when that's all that's available.
function Ensure-MinGW {
    if ((Test-Have "as") -and (Test-Have "dlltool")) {
        Write-Info "MinGW-w64 toolchain already present."
        return
    }
    Write-Info "Installing MinGW-w64 (C toolchain needed to build librespot)..."

    $mpm = $PM
    if ($mpm -eq "winget" -and -not (Test-Have "scoop")) {
        Write-Info "Bootstrapping scoop (no admin) to obtain MinGW-w64..."
        try {
            Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
            Update-Path
            if (Test-Have "scoop") { $mpm = "scoop" }
        } catch { Write-Warn "scoop bootstrap failed: $_" }
    }

    switch ($mpm) {
        "scoop" {
            scoop bucket add main
            scoop install gcc
            $gccBin = Join-Path $env:USERPROFILE "scoop\apps\gcc\current\bin"
            if (Test-Path $gccBin) { $env:PATH = "$gccBin;" + $env:PATH }
        }
        "choco"  { choco install mingw -y }
        default  { Write-Warn "No package manager can install MinGW-w64 automatically." }
    }
    Update-Path
}

# Install-Librespot builds and installs librespot from source, picking the MSVC
# path when its linker is available and falling back to the no-admin GNU path.
function Install-Librespot {
    if (Test-Have "librespot") { Write-Info "librespot already installed."; return }

    Write-Info "Installing librespot (for local PC playback)..."

    if (-not (Test-Have "cargo")) {
        Write-Info "Installing the Rust toolchain (needed to build librespot)..."
        switch ($PM) {
            "winget" { winget install --id Rustlang.Rustup -e --accept-source-agreements --accept-package-agreements }
            "scoop"  { scoop install rustup; rustup default stable }
            "choco"  { choco install rustup.install -y }
            default  { Write-Warn "Install Rust from https://rustup.rs, then re-run."; return }
        }
        Update-Path
    }
    if (-not (Test-Have "cargo")) {
        Write-Warn "cargo still not on PATH. Open a new terminal and re-run, or install Rust from https://rustup.rs."
        return
    }

    if (Test-Have "link") {
        # MSVC linker present (Visual Studio Build Tools) — the default target builds.
        Write-Info "Compiling librespot with the MSVC toolchain (this may take a few minutes)..."
        try { cargo install librespot --locked } catch {}
    } else {
        Write-Info "No MSVC linker (link.exe) found — building with the GNU toolchain (no admin needed)."
        rustup toolchain install stable-x86_64-pc-windows-gnu
        Ensure-MinGW
        if (-not ((Test-Have "as") -and (Test-Have "dlltool"))) {
            Write-Warn "Couldn't set up a MinGW-w64 toolchain (need as.exe + dlltool.exe)."
            Write-Warn "Install one (e.g. 'scoop install gcc' or 'choco install mingw -y') and re-run."
            return
        }
        Write-Info "Compiling librespot with the GNU toolchain (this may take a few minutes)..."
        try { cargo +stable-x86_64-pc-windows-gnu install librespot --locked } catch {}
    }
    Update-Path

    if (Test-Have "librespot") {
        Write-Info "librespot installed."
    } else {
        Write-Warn "Couldn't install librespot automatically."
        Write-Warn "SpoTUI will still build and run, but without local PC playback."
        Write-Warn "Install it later with:  cargo install librespot --locked   (needs Rust from https://rustup.rs)"
    }
}

if ($SkipLibrespot) {
    Write-Warn "Skipping librespot install (-SkipLibrespot). SpoTUI will run in control-only mode."
} else {
    Install-Librespot
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
