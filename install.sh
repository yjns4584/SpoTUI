#!/usr/bin/env bash
#
# SpoTUI installer — installs Go and spotifyd if missing, builds the binary,
# and places it on your PATH so you can launch it by typing `spotui`.
#
# Usage (from inside the cloned repo):
#   ./install.sh
#
set -euo pipefail

# ── Output helpers ─────────────────────────────────────────────────────────
BOLD=$'\e[1m'; GREEN=$'\e[32m'; YELLOW=$'\e[33m'; RED=$'\e[31m'; RESET=$'\e[0m'
info()  { echo "${GREEN}==>${RESET} ${BOLD}$*${RESET}"; }
warn()  { echo "${YELLOW}!!${RESET} $*"; }
err()   { echo "${RED}xx${RESET} $*" >&2; }
have()  { command -v "$1" >/dev/null 2>&1; }

# ── Must run from the repo root ────────────────────────────────────────────
if [ ! -f go.mod ] || [ ! -d cmd/spotui ]; then
  err "Run this script from the SpoTUI repo root (where go.mod lives)."
  exit 1
fi

# ── Detect package manager ─────────────────────────────────────────────────
OS="$(uname -s)"
PM=""
if [ "$OS" = "Darwin" ]; then
  PM="brew"
elif have pacman; then PM="pacman"
elif have apt-get; then PM="apt"
elif have dnf;     then PM="dnf"
elif have zypper;  then PM="zypper"
fi

if [ -z "$PM" ]; then
  warn "No supported package manager detected."
  warn "Install Go (https://go.dev/dl) and spotifyd (https://github.com/Spotifyd/spotifyd) manually, then re-run."
  PM="none"
fi

pm_install() {
  case "$PM" in
    brew)   brew install "$@" ;;
    pacman) sudo pacman -S --needed --noconfirm "$@" ;;
    apt)    sudo apt-get update && sudo apt-get install -y "$@" ;;
    dnf)    sudo dnf install -y "$@" ;;
    zypper) sudo zypper install -y "$@" ;;
    none)   return 1 ;;
  esac
}

# ── Ensure Go ──────────────────────────────────────────────────────────────
if have go; then
  info "Go already installed ($(go version | awk '{print $3}'))."
else
  info "Installing Go..."
  case "$PM" in
    apt)         pm_install golang-go ;;
    dnf|zypper)  pm_install golang ;;
    *)           pm_install go ;;
  esac
fi
if ! have go; then
  err "Go is still not on PATH. Install it from https://go.dev/dl and re-run."
  exit 1
fi

# ── Ensure spotifyd (local audio playback) ─────────────────────────────────
if have spotifyd; then
  info "spotifyd already installed."
else
  info "Installing spotifyd..."
  if ! pm_install spotifyd; then
    warn "Couldn't install spotifyd automatically with this package manager."
    warn "Grab a binary from https://github.com/Spotifyd/spotifyd/releases and put it on your PATH."
    warn "SpoTUI will still build; you just won't have local playback until spotifyd is installed."
  fi
fi

# ── Build ──────────────────────────────────────────────────────────────────
info "Building spotui..."
go build -o spotui ./cmd/spotui

# ── Install on PATH ────────────────────────────────────────────────────────
INSTALL_DIR="/usr/local/bin"
info "Installing to $INSTALL_DIR (may ask for your password)..."
sudo install -m 755 spotui "$INSTALL_DIR/spotui"
rm -f spotui

echo
info "Done! 🎵"
echo "  Launch it from any terminal:  ${BOLD}spotui${RESET}"
echo
echo "  Requirements:"
echo "   • Spotify Premium account"
echo "   • First run opens your browser twice to log in (control + spotifyd)"
if ! have spotifyd; then
  warn "spotifyd is not installed yet — install it for local PC playback."
fi