#!/bin/sh
set -e

REPO="bluefunda/abaper"
BINARY="abaper"
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

# Resolve install directory
if [ -n "$ABAPER_INSTALL_DIR" ]; then
  INSTALL_DIR="$ABAPER_INSTALL_DIR"
elif [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# Check dependencies
REQUIRED_CMDS="curl sha256sum"
[ "$OS" = "darwin" ] && REQUIRED_CMDS="curl shasum unzip"
for cmd in $REQUIRED_CMDS; do
  command -v "$cmd" >/dev/null 2>&1 || die "'$cmd' is required but not installed"
done

# Fetch latest version
info "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')
[ -n "$VERSION" ] || die "Could not determine latest version"

# darwin uses zip, linux uses tar.gz
if [ "$OS" = "darwin" ]; then
  ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.zip"
else
  ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
fi
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

# Extract and install
if [ "$OS" = "darwin" ]; then
  unzip -q "$ARCHIVE" "$BINARY"
else
  tar -xzf "$ARCHIVE" "$BINARY"
fi

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

printf "\n${GREEN}${BOLD}ABAPer CLI installed!${RESET}\n"
printf "  Run: ${BOLD}${BINARY} --help${RESET}\n\n"
