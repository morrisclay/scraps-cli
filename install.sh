#!/bin/bash
set -e

# Scraps CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/morrisclay/scraps-cli/master/install.sh | bash

REPO="morrisclay/scraps-cli"
BINARY_NAME="scraps"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux*)  OS="linux" ;;
    darwin*) OS="darwin" ;;
    msys*|mingw*|cygwin*) OS="windows" ;;
    *)
        echo "Error: Unsupported operating system: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Get latest version from GitHub API
echo "Fetching latest release..."
LATEST_RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
    echo "Error: Could not determine latest release"
    exit 1
fi

VERSION="${LATEST_RELEASE#v}"
echo "Latest version: ${VERSION}"

# Construct download URL
if [ "$OS" = "windows" ]; then
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.zip"
else
    ARCHIVE_NAME="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${ARCHIVE_NAME}"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${ARCHIVE_NAME}..."
curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"

# Extract archive
echo "Extracting..."
cd "$TMP_DIR"
if [ "$OS" = "windows" ]; then
    unzip -q "$ARCHIVE_NAME"
else
    tar -xzf "$ARCHIVE_NAME"
fi

# Install binary
echo "Installing to ${INSTALL_DIR}..."
if [ -w "$INSTALL_DIR" ]; then
    mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    sudo mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "Successfully installed ${BINARY_NAME} ${VERSION} to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Run 'scraps --help' to get started"
