#!/bin/bash
# Build script for BingDork Pro
# Usage: ./scripts/build.sh [version]

set -euo pipefail

APP_NAME="bingdork"
VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILD_DIR="build"

echo "Building ${APP_NAME} v${VERSION}..."
echo "Commit: ${COMMIT}"
echo "Date: ${DATE}"
echo ""

LDFLAGS="-ldflags "
LDFLAGS+="-X github.com/bingdork/bingdork/cli.Version=${VERSION} "
LDFLAGS+="-X github.com/bingdork/bingdork/cli.Commit=${COMMIT} "
LDFLAGS+="-X github.com/bingdork/bingdork/cli.Date=${DATE} "
LDFLAGS+="-X github.com/bingdork/bingdork/cli.GoVersion=$(go version | awk '{print $3}')"

mkdir -p "${BUILD_DIR}"

# Build for current platform
echo "Building for $(go env GOOS)/$(go env GOARCH)..."
CGO_ENABLED=0 go build ${LDFLAGS} -o "${BUILD_DIR}/${APP_NAME}" "./cmd/${APP_NAME}/"
echo "Built: ${BUILD_DIR}/${APP_NAME}"
echo ""

# Build for all platforms if --all flag
if [ "${2:-}" = "--all" ]; then
    echo "Cross-compiling for all platforms..."
    
    platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r GOOS GOARCH <<< "${platform}"
        ext=""
        if [ "${GOOS}" = "windows" ]; then
            ext=".exe"
        fi
        
        output="${BUILD_DIR}/${APP_NAME}-${GOOS}-${GOARCH}${ext}"
        echo "  Building for ${GOOS}/${GOARCH}..."
        
        CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build ${LDFLAGS} -o "${output}" "./cmd/${APP_NAME}/"
    done
    
    echo ""
    echo "Cross-compilation complete."
fi

echo "Build complete."
ls -lh "${BUILD_DIR}/"
