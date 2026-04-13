#!/usr/bin/env bash
# devkit CLI installer for Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/scripts/install.sh | bash
set -euo pipefail

REPO="stichting-Cyberbrein-nl/ctfdevkit-cli"
BINARY_NAME="devkit"
INSTALL_DIR="${DEVKIT_INSTALL_DIR:-/usr/local/bin}"

# ── Color helpers ────────────────────────────────────────────────────────────
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RESET='\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; CYAN=''; RESET=''
fi
info()    { echo -e "${CYAN}  ●${RESET} $*"; }
success() { echo -e "${GREEN}  ✓${RESET} $*"; }
warn()    { echo -e "${YELLOW}  ⚠${RESET} $*"; }
fail()    { echo -e "${RED}  ✗${RESET} $*" >&2; exit 1; }

# ── Detect OS + arch ────────────────────────────────────────────────────────
detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os="darwin" ;;
    Linux)  os="linux"  ;;
    *)      fail "Unsupported OS: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) fail "Unsupported architecture: $(uname -m)" ;;
  esac
  echo "${os}-${arch}"
}

# ── Fetch latest release version from GitHub ────────────────────────────────
latest_version() {
  if command -v curl &>/dev/null; then
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/'
  elif command -v wget &>/dev/null; then
    wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/'
  else
    fail "curl or wget is required"
  fi
}

# ── Download a file ──────────────────────────────────────────────────────────
download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL --progress-bar -o "$dest" "$url"
  else
    wget -q --show-progress -O "$dest" "$url"
  fi
}

# ── Verify SHA256 checksum ───────────────────────────────────────────────────
verify_checksum() {
  local file="$1" expected="$2"
  local actual
  if command -v sha256sum &>/dev/null; then
    actual=$(sha256sum "$file" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    actual=$(shasum -a 256 "$file" | awk '{print $1}')
  else
    warn "No checksum tool found — skipping verification"
    return 0
  fi
  if [ "$actual" != "$expected" ]; then
    fail "Checksum mismatch: expected $expected, got $actual"
  fi
  success "Checksum verified"
}

# ── Main ─────────────────────────────────────────────────────────────────────
main() {
  echo ""
  echo "  ██████╗ ███████╗██╗   ██╗██╗  ██╗██╗████████╗"
  echo "  ██╔══██╗██╔════╝██║   ██║██║ ██╔╝██║╚══██╔══╝"
  echo "  ██║  ██║█████╗  ██║   ██║█████╔╝ ██║   ██║   "
  echo "  ██║  ██║██╔══╝  ╚██╗ ██╔╝██╔═██╗ ██║   ██║   "
  echo "  ██████╔╝███████╗ ╚████╔╝ ██║  ██╗██║   ██║   "
  echo "  ╚═════╝ ╚══════╝  ╚═══╝  ╚═╝  ╚═╝╚═╝   ╚═╝   "
  echo ""
  echo "  Cyberbrein DevKit Installer"
  echo ""

  local platform version
  platform=$(detect_platform)
  version=$(latest_version)

  info "Platform: ${platform}"
  info "Version:  v${version}"
  info "Installing to: ${INSTALL_DIR}"
  echo ""

  local base_url="https://github.com/${REPO}/releases/download/v${version}"
  local archive_name="${BINARY_NAME}-${platform}.tar.gz"
  local checksum_url="${base_url}/checksums.txt"
  local archive_url="${base_url}/${archive_name}"

  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  info "Downloading ${archive_name}..."
  download "$archive_url" "${tmpdir}/${archive_name}"

  # Fetch and verify checksum.
  info "Verifying checksum..."
  download "$checksum_url" "${tmpdir}/checksums.txt"
  local expected
  expected=$(grep "${archive_name}" "${tmpdir}/checksums.txt" | awk '{print $1}')
  verify_checksum "${tmpdir}/${archive_name}" "$expected"

  # Extract.
  info "Extracting..."
  tar -xzf "${tmpdir}/${archive_name}" -C "${tmpdir}"

  # Install.
  if [ ! -w "${INSTALL_DIR}" ]; then
    info "Requires sudo to install to ${INSTALL_DIR}..."
    sudo install -m 755 "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    install -m 755 "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  fi

  echo ""
  success "devkit v${version} installed to ${INSTALL_DIR}/${BINARY_NAME}"
  echo ""
  echo "  Get started:"
  echo "    devkit setup"
  echo ""
}

main "$@"
