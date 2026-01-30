#!/bin/sh
# Scraps CLI installer script
# Usage: curl -sSL https://scraps.sh/install.sh | sh

set -e

REPO="scraps-sh/scraps-cli"
BINARY_NAME="scraps"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo "Error: Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo "Error: Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    echo "${OS}_${ARCH}"
}

# Get latest version from GitHub
get_latest_version() {
    curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name":' |
        sed -E 's/.*"([^"]+)".*/\1/' |
        sed 's/^v//'
}

# Download and install
install() {
    PLATFORM=$(detect_platform)
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi

    echo "Installing scraps v${VERSION} for ${PLATFORM}..."

    # Construct download URL
    EXT="tar.gz"
    if [ "$OS" = "windows" ]; then
        EXT="zip"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/scraps_${VERSION}_${PLATFORM}.${EXT}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download
    echo "Downloading from ${DOWNLOAD_URL}..."
    curl -sSL "$DOWNLOAD_URL" -o "$TMP_DIR/scraps.${EXT}"

    # Extract
    cd "$TMP_DIR"
    if [ "$EXT" = "zip" ]; then
        unzip -q "scraps.${EXT}"
    else
        tar -xzf "scraps.${EXT}"
    fi

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY_NAME" "$INSTALL_DIR/"
    else
        echo "Installing to $INSTALL_DIR requires sudo..."
        sudo mv "$BINARY_NAME" "$INSTALL_DIR/"
    fi

    chmod +x "$INSTALL_DIR/$BINARY_NAME"

    echo ""
    echo "✓ scraps v${VERSION} installed successfully!"
    echo ""
    echo "Get started:"
    echo "  scraps login      # Authenticate with your API key"
    echo "  scraps --help     # See all commands"
    echo ""
}

install
