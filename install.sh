#!/bin/sh
# peek installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | sh
# Or with custom options:
#   curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | INSTALL_DIR=/usr/local/bin sh
#   curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | VERSION=v0.1.1 sh

set -e

REPO="bimalpaudels/peep"
BINARY="peek"

# Terminal colors
if [ -t 1 ]; then
    RED="\033[0;31m"
    GREEN="\033[0;32m"
    BLUE="\033[0;34m"
    YELLOW="\033[1;33m"
    BOLD="\033[1m"
    RESET="\033[0m"
else
    RED=""
    GREEN=""
    BLUE=""
    YELLOW=""
    BOLD=""
    RESET=""
fi

info() {
    printf "${BLUE}==>${RESET} ${BOLD}%s${RESET}\n" "$1"
}

success() {
    printf "${GREEN}✓${RESET} %s\n" "$1"
}

warn() {
    printf "${YELLOW}!${RESET} %s\n" "$1"
}

error() {
    printf "${RED}x Error:${RESET} %s\n" "$1" >&2
    exit 1
}

# 1. Detect OS
OS_RAW=$(uname -s)
case "$OS_RAW" in
    Darwin)
        OS="darwin"
        ;;
    Linux)
        OS="linux"
        ;;
    *)
        error "Unsupported operating system: $OS_RAW. peek currently supports macOS and Linux via this script."
        ;;
esac

# 2. Detect Architecture
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        error "Unsupported architecture: $ARCH_RAW. Supported architectures: x86_64, arm64."
        ;;
esac

ASSET_NAME="peek-${OS}-${ARCH}"

# 3. Determine Version & Download URL
if [ -n "$VERSION" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
    TAG_DISPLAY="$VERSION"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
    TAG_DISPLAY="latest"
fi

# 4. Determine Installation Directory
if [ -z "$INSTALL_DIR" ]; then
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="${HOME}/.local/bin"
    fi
fi

info "Installing peek (${TAG_DISPLAY}) for ${OS}/${ARCH}..."

# 5. Download binary to a temporary file
TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'peek-install')
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

TMP_FILE="${TMP_DIR}/${BINARY}"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE" || error "Failed to download $DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$TMP_FILE" "$DOWNLOAD_URL" || error "Failed to download $DOWNLOAD_URL"
else
    error "Neither 'curl' nor 'wget' was found. Please install curl or wget."
fi

chmod +x "$TMP_FILE"

# 6. Install to target directory
mkdir -p "$INSTALL_DIR" || error "Could not create target directory: $INSTALL_DIR"

if [ -w "$INSTALL_DIR" ]; then
    mv -f "$TMP_FILE" "${INSTALL_DIR}/${BINARY}"
else
    info "Elevated permissions required to write to $INSTALL_DIR..."
    sudo mv -f "$TMP_FILE" "${INSTALL_DIR}/${BINARY}"
fi

success "Installed peek to ${INSTALL_DIR}/${BINARY}"

# 7. Check PATH
case ":$PATH:" in
    *":${INSTALL_DIR}:"*)
        ;;
    *)
        echo ""
        warn "${INSTALL_DIR} is not in your PATH."
        printf "  Add it to your shell profile (~/.zshrc or ~/.bashrc):\n"
        printf "    ${BOLD}export PATH=\"%s:\$PATH\"${RESET}\n\n" "$INSTALL_DIR"
        ;;
esac

# 8. Check uv requirement
if ! command -v uv >/dev/null 2>&1; then
    echo ""
    warn "peek requires 'uv' (astral.sh/uv) to execute Python code, but it was not detected."
    printf "  Install uv with:\n"
    if [ "$OS" = "darwin" ] && command -v brew >/dev/null 2>&1; then
        printf "    ${BOLD}brew install uv${RESET}\n"
        printf "    or: ${BOLD}curl -LsSf https://astral.sh/uv/install.sh | sh${RESET}\n\n"
    else
        printf "    ${BOLD}curl -LsSf https://astral.sh/uv/install.sh | sh${RESET}\n\n"
    fi
fi

# 9. Verify installation
if command -v peek >/dev/null 2>&1; then
    INSTALLED_VER=$(peek --version 2>/dev/null || true)
    if [ -n "$INSTALLED_VER" ]; then
        success "peek ($INSTALLED_VER) is ready to use!"
    else
        success "peek is ready to use!"
    fi
fi
