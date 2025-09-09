# Beenet PowerShell Build Script
# Alternative to Makefile for Windows PowerShell environments

param(
    [string]$Target = "build"
)

# Build configuration
$BINARY_NAME = "bee"
$BUILD_DIR = "build"
$DIST_DIR = "dist"
$COVERAGE_DIR = "coverage"

# Get version info with error handling
try {
    $VERSION = git describe --tags --always --dirty 2>$null
    if (-not $VERSION) { $VERSION = "dev" }
} catch {
    $VERSION = "dev"
}

try {
    $COMMIT_HASH = git rev-parse --short HEAD 2>$null
    if (-not $COMMIT_HASH) { $COMMIT_HASH = "unknown" }
} catch {
    $COMMIT_HASH = "unknown"
}

# Get current timestamp in ISO format
$BUILD_TIME = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

# Go configuration
$GO = "go"
$GOFLAGS = "-trimpath"
$LDFLAGS = "-s -w -X main.version=$VERSION -X main.buildTime=$BUILD_TIME -X main.commitHash=$COMMIT_HASH"

function Build {
    Write-Host "Building $BINARY_NAME $VERSION..." -ForegroundColor Green
    
    # Create build directory
    if (-not (Test-Path $BUILD_DIR)) {
        New-Item -ItemType Directory -Path $BUILD_DIR -Force | Out-Null
    }
    
    # Build the binary
    $buildCmd = "$GO build $GOFLAGS -ldflags `"$LDFLAGS`" -o $BUILD_DIR/$BINARY_NAME.exe ./cmd/bee"
    Write-Host "Executing: $buildCmd" -ForegroundColor Yellow
    
    Invoke-Expression $buildCmd
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build successful: $BUILD_DIR/$BINARY_NAME.exe" -ForegroundColor Green
    } else {
        Write-Host "Build failed with error code $LASTEXITCODE" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

function Clean {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Green
    
    if (Test-Path $BUILD_DIR) { Remove-Item -Recurse -Force $BUILD_DIR }
    if (Test-Path $DIST_DIR) { Remove-Item -Recurse -Force $DIST_DIR }
    if (Test-Path $COVERAGE_DIR) { Remove-Item -Recurse -Force $COVERAGE_DIR }
    
    & $GO clean -cache -testcache -modcache
}

function Test {
    Write-Host "Running tests..." -ForegroundColor Green
    & $GO test -v ./...
}

function Format {
    Write-Host "Formatting code..." -ForegroundColor Green
    & $GO fmt ./...
}

function Vet {
    Write-Host "Vetting code..." -ForegroundColor Green
    & $GO vet ./...
}

function ShowHelp {
    Write-Host "Beenet PowerShell Build Script" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\build.ps1 [target]" -ForegroundColor White
    Write-Host ""
    Write-Host "Available targets:" -ForegroundColor White
    Write-Host "  build    - Build the binary (default)" -ForegroundColor Gray
    Write-Host "  clean    - Clean build artifacts" -ForegroundColor Gray
    Write-Host "  test     - Run tests" -ForegroundColor Gray
    Write-Host "  fmt      - Format code" -ForegroundColor Gray
    Write-Host "  vet      - Vet code" -ForegroundColor Gray
    Write-Host "  help     - Show this help" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Build info:" -ForegroundColor White
    Write-Host "  Version:     $VERSION" -ForegroundColor Gray
    Write-Host "  Build time:  $BUILD_TIME" -ForegroundColor Gray
    Write-Host "  Commit:      $COMMIT_HASH" -ForegroundColor Gray
}

# Main execution
switch ($Target.ToLower()) {
    "build" { Build }
    "clean" { Clean }
    "test" { Test }
    "fmt" { Format }
    "vet" { Vet }
    "help" { ShowHelp }
    default { 
        Write-Host "Unknown target: $Target" -ForegroundColor Red
        ShowHelp
        exit 1
    }
}
