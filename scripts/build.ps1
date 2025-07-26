# Build script for ForgeCache with version information

$ErrorActionPreference = "Stop"

# Get build information
$COMMIT = git rev-parse HEAD
$COMMIT_SHORT = git rev-parse --short HEAD
$DATE = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$GO_VERSION = (go version).Split(' ')[2]
$BUILD_OS = go env GOOS
$BUILD_ARCH = go env GOARCH

# Determine version
if ($env:VERSION) {
    $BUILD_VERSION = $env:VERSION
} else {
    $BUILD_VERSION = "dev-$COMMIT_SHORT"
}

Write-Host "Building ForgeCache $BUILD_VERSION"
Write-Host "Commit: $COMMIT_SHORT"
Write-Host "Date: $DATE"
Write-Host "Go: $GO_VERSION"
Write-Host "OS/Arch: $BUILD_OS/$BUILD_ARCH"

# Build with version information
$ldflags = "-w -s -X main.version=$BUILD_VERSION -X main.commit=$COMMIT -X main.date=$DATE -X main.goVersion=$GO_VERSION -X main.buildOS=$BUILD_OS -X main.buildArch=$BUILD_ARCH"

go build -v -ldflags="$ldflags" -o forge.exe ./cmd/forge

Write-Host "✅ Build complete: forge.exe"
Write-Host "Run '.\forge.exe version' to see build information"
