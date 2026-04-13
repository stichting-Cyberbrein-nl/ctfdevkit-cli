#!/usr/bin/env bash
# Cyberbrein DevKit — Linux installer (Debian / Ubuntu / Kali)
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/scripts/install.sh | bash
#
# Override the install directory:
#   DEVKIT_INSTALL_DIR=~/.local/bin bash <(curl -fsSL ...)
set -euo pipefail

REPO="stichting-Cyberbrein-nl/ctfdevkit-cli"
BINARY="devkit"

# ── Colors ────────────────────────────────────────────────────────────────────
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  R='\033[0;31m' G='\033[0;32m' Y='\033[1;33m' C='\033[0;36m' B='\033[1m' N='\033[0m'
else
  R='' G='' Y='' C='' B='' N=''
fi
info()    { echo -e "${C}  ●${N} $*"; }
ok()      { echo -e "${G}  ✓${N} $*"; }
warn()    { echo -e "${Y}  ⚠${N} $*"; }
die()     { echo -e "${R}  ✗${N} $*" >&2; exit 1; }

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
echo -e "${B}  ██████╗ ███████╗██╗   ██╗██╗  ██╗██╗████████╗${N}"
echo -e "${B}  ██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║╚══██╔══╝${N}"
echo -e "${B}  ██║  ██║█████╗  ██║   ██║█████╔╝ ██║   ██║   ${N}"
echo -e "${B}  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔═██╗ ██║   ██║   ${N}"
echo -e "${B}  ██████╔╝███████╗ ╚████╔╝ ██║  ██╗██║   ██║   ${N}"
echo -e "${B}  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝╚═╝   ╚═╝   ${N}"
echo ""
echo "  Cyberbrein DevKit — Linux Installer"
echo ""

# ── OS check ──────────────────────────────────────────────────────────────────
[[ "$(uname -s)" == "Linux" ]] || die "This script is for Linux only (Debian / Ubuntu / Kali)."

# ── Architecture ──────────────────────────────────────────────────────────────
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac
PLATFORM="linux-${ARCH}"

# ── Require curl or wget ──────────────────────────────────────────────────────
if command -v curl &>/dev/null; then
  fetch()      { curl -fsSL "$1"; }
  fetch_file() { curl -fsSL --progress-bar -o "$2" "$1"; }
elif command -v wget &>/dev/null; then
  fetch()      { wget -qO- "$1"; }
  fetch_file() { wget -q --show-progress -O "$2" "$1"; }
else
  die "curl or wget is required. Install with: sudo apt-get install -y curl"
fi

# ── Fetch latest release version ──────────────────────────────────────────────
info "Fetching latest version..."
VERSION=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
[[ -n "$VERSION" ]] || die "Could not determine latest version. Check your internet connection."

# ── Decide install directory ───────────────────────────────────────────────────
if [[ -n "${DEVKIT_INSTALL_DIR:-}" ]]; then
  INSTALL_DIR="$DEVKIT_INSTALL_DIR"
elif [[ $EUID -eq 0 ]] || sudo -n true 2>/dev/null; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

info "Platform:   ${PLATFORM}"
info "Version:    v${VERSION}"
info "Installing to: ${INSTALL_DIR}"
echo ""

# ── Download ──────────────────────────────────────────────────────────────────
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
ARCHIVE="${BINARY}-${PLATFORM}.tar.gz"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading ${ARCHIVE}..."
fetch_file "${BASE_URL}/${ARCHIVE}"       "${TMPDIR}/${ARCHIVE}"
fetch_file "${BASE_URL}/checksums.txt"    "${TMPDIR}/checksums.txt"

# ── Verify checksum ───────────────────────────────────────────────────────────
info "Verifying checksum..."
EXPECTED=$(grep "${ARCHIVE}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
if command -v sha256sum &>/dev/null; then
  ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
else
  ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
fi
[[ "$ACTUAL" == "$EXPECTED" ]] || die "Checksum mismatch — download may be corrupted."
ok "Checksum verified"

# ── Extract ───────────────────────────────────────────────────────────────────
info "Extracting..."
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}"

# ── Install binary ────────────────────────────────────────────────────────────
mkdir -p "${INSTALL_DIR}"

if [[ -w "${INSTALL_DIR}" ]]; then
  install -m 755 "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  info "Elevated privileges required to write to ${INSTALL_DIR}..."
  sudo install -m 755 "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

ok "devkit v${VERSION} installed at ${INSTALL_DIR}/${BINARY}"

# ── PATH setup ────────────────────────────────────────────────────────────────
ensure_in_path() {
  local dir="$1"
  local line="export PATH=\"${dir}:\$PATH\""
  local added=0

  for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
    [[ -f "$rc" ]] || continue
    grep -qF "$dir" "$rc" && continue
    printf '\n# Cyberbrein DevKit\n%s\n' "$line" >> "$rc"
    added=1
  done

  if [[ $added -eq 1 ]]; then
    warn "${dir} added to your shell config."
    warn "Open a new terminal or run:  source ~/.bashrc"
  fi
}

if printf '%s\n' "${PATH//:/$'\n'}" | grep -qxF "${INSTALL_DIR}"; then
  ok "${INSTALL_DIR} is already in your PATH"
else
  info "Updating PATH..."
  ensure_in_path "${INSTALL_DIR}"
fi

# ── Smoke test ────────────────────────────────────────────────────────────────
INSTALLED_VER=$("${INSTALL_DIR}/${BINARY}" version 2>/dev/null || true)
[[ -n "$INSTALLED_VER" ]] && ok "Smoke test passed: ${INSTALLED_VER}"

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ┌──────────────────────────────────────────┐"
echo "  │  Get started:                            │"
echo "  │    devkit setup                          │"
echo "  │    devkit up                             │"
echo "  │                                          │"
echo "  │  Update later:                           │"
echo "  │    devkit self-update                    │"
echo "  └──────────────────────────────────────────┘"
echo ""
