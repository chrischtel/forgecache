#!/bin/bash

# Build script for ForgeCache with version information

set -e

# Get build information
COMMIT=$(git rev-parse HEAD)
COMMIT_SHORT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION=$(go version | cut -d' ' -f3)
BUILD_OS=$(go env GOOS)
BUILD_ARCH=$(go env GOARCH)

# Determine version
if [ -n "$VERSION" ]; then
    # Use provided version (for releases)
    BUILD_VERSION="$VERSION"
else
    # Use dev version with commit hash
    BUILD_VERSION="dev-$COMMIT_SHORT"
fi

echo "Building ForgeCache $BUILD_VERSION"
echo "Commit: $COMMIT_SHORT"
echo "Date: $DATE"
echo "Go: $GO_VERSION"
echo "OS/Arch: $BUILD_OS/$BUILD_ARCH"

# Build with version information
go build -v -ldflags="-w -s \
    -X main.version=$BUILD_VERSION \
    -X main.commit=$COMMIT \
    -X main.date=$DATE \
    -X main.goVersion=$GO_VERSION \
    -X main.buildOS=$BUILD_OS \
    -X main.buildArch=$BUILD_ARCH" \
    -o forge ./cmd/forge

echo "✅ Build complete: forge"
echo "Run './forge version' to see build information"
