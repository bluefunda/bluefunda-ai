#!/bin/sh
set -e

REPO="bluefunda/bluefunda-ai"
BINARY="bai"
INSTALL_DIR=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { printf "${BLUE}==>${RESET} ${BOLD}%s${RESET}\n" "$*"; }
ok()    { printf "${GREEN} ✓${RESET} %s\n" "$*"; }
die()   { printf "${RED}error:${RESET} %s\n" "$*" >&2; exit 1; }

# Detect OS
case "$(uname -s)" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      die "Unsupported OS: $(uname -s)" ;;
esac

# Detect architecture
case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac

# Pick archive format
if [ "$OS" = "darwin" ]; then
  EXT="zip"
else
  EXT="tar.gz"
fi

# Resolve install directory
if [ -n "$BAI_INSTALL_DIR" ]; then
  INSTALL_DIR="$BAI_INSTALL_DIR"
elif [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# Check dependencies
for cmd in curl; do
  command -v "$cmd" >/dev/null 2>&1 || die "'$cmd' is required but not installed"
done
if [ "$OS" = "darwin" ]; then
  command -v shasum >/dev/null 2>&1 || die "'shasum' is required but not installed"
else
  command -v sha256sum >/dev/null 2>&1 || die "'sha256sum' is required but not installed"
fi
if [ "$EXT" = "zip" ]; then
  command -v unzip >/dev/null 2>&1 || die "'unzip' is required but not installed"
fi

# Fetch latest version
info "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')
[ -n "$VERSION" ] || die "Could not determine latest version"

ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

info "Installing ${BOLD}${BINARY}${RESET} v${VERSION} (${OS}/${ARCH})..."

# Download archive and checksums
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "${BASE_URL}/${ARCHIVE}"     -o "${TMPDIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt"  -o "${TMPDIR}/checksums.txt"

# Verify checksum
cd "$TMPDIR"
if [ "$OS" = "darwin" ]; then
  grep "${ARCHIVE}" checksums.txt | shasum -a 256 -c --quiet || die "Checksum verification failed"
else
  grep "${ARCHIVE}" checksums.txt | sha256sum -c --quiet || die "Checksum verification failed"
fi
ok "Checksum verified"

# Extract
if [ "$EXT" = "zip" ]; then
  unzip -q "$ARCHIVE" "$BINARY"
else
  tar -xzf "$ARCHIVE" "$BINARY"
fi

# Install
if [ "$INSTALL_DIR" = "/usr/local/bin" ] && [ ! -w "/usr/local/bin" ]; then
  sudo install -m 755 "$BINARY" "${INSTALL_DIR}/${BINARY}"
else
  install -m 755 "$BINARY" "${INSTALL_DIR}/${BINARY}"
fi

ok "Installed to ${INSTALL_DIR}/${BINARY}"

# PATH hint for ~/.local/bin
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) printf "\n${BOLD}Add to your shell profile:${RESET}\n  export PATH=\"\$HOME/.local/bin:\$PATH\"\n" ;;
  esac
fi

printf "\n${GREEN}${BOLD}BlueFunda AI CLI installed!${RESET}\n"
printf "  Run: ${BOLD}${BINARY} login${RESET}\n\n"
