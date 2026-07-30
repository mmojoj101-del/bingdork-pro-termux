#!/bin/bash
# Install script for BingDork Pro
# Usage: curl -sSL https://raw.githubusercontent.com/bingdork/bingdork/main/scripts/install.sh | bash

set -euo pipefail

APP_NAME="bingdork"
REPO="bingdork/bingdork"
VERSION="${1:-latest}"

echo "Installing ${APP_NAME}..."

# Determine OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: ${ARCH}"; exit 1 ;;
esac

# Determine install directory
INSTALL_DIR="${BINGDORK_INSTALL_DIR:-/usr/local/bin}"

# Download URL
if [ "${VERSION}" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${APP_NAME}-${OS}-${ARCH}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${APP_NAME}-${OS}-${ARCH}"
fi

echo "Downloading ${APP_NAME} ${VERSION} for ${OS}/${ARCH}..."
echo "URL: ${DOWNLOAD_URL}"

# Download binary
if command -v curl &> /dev/null; then
    curl -sSL "${DOWNLOAD_URL}" -o "${INSTALL_DIR}/${APP_NAME}"
elif command -v wget &> /dev/null; then
    wget -q "${DOWNLOAD_URL}" -O "${INSTALL_DIR}/${APP_NAME}"
else
    echo "Error: curl or wget required"
    exit 1
fi

# Make executable
chmod +x "${INSTALL_DIR}/${APP_NAME}"

echo ""
echo "${APP_NAME} installed to ${INSTALL_DIR}/${APP_NAME}"
echo ""
echo "Run '${APP_NAME} --help' to get started"
echo "Run '${APP_NAME} doctor' to check system health"
echo "Run '${APP_NAME} config --init' to create default configuration"
