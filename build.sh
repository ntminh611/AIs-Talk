#!/bin/bash

# Build script for AI Multi-Agent Debate
# Builds binaries for macOS, Linux, and Windows

set -e

VERSION=${1:-"v1.0.0"}
OUTPUT_DIR="dist"
BINARY_NAME="talk"

echo "🔨 Building AI Multi-Agent Debate ${VERSION}..."
echo ""

# Clean and create output directory
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# Build targets
TARGETS=(
    "darwin/arm64"    # macOS Apple Silicon
    "darwin/amd64"    # macOS Intel
    "linux/amd64"     # Linux x64
    "linux/arm64"     # Linux ARM64
    "windows/amd64"   # Windows x64
)

for TARGET in "${TARGETS[@]}"; do
    OS=$(echo $TARGET | cut -d'/' -f1)
    ARCH=$(echo $TARGET | cut -d'/' -f2)
    
    OUTPUT_NAME="${BINARY_NAME}-${OS}-${ARCH}"
    
    if [ "$OS" == "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo "📦 Building ${OUTPUT_NAME}..."
    
    GOOS=$OS GOARCH=$ARCH go build -ldflags="-s -w -X main.Version=${VERSION}" -o "${OUTPUT_DIR}/${OUTPUT_NAME}" .
    
    if [ $? -eq 0 ]; then
        SIZE=$(ls -lh "${OUTPUT_DIR}/${OUTPUT_NAME}" | awk '{print $5}')
        echo "   ✅ Done! Size: ${SIZE}"
    else
        echo "   ❌ Failed!"
        exit 1
    fi
done

echo ""
echo "🎉 All builds completed!"
echo ""
echo "📂 Output directory: ${OUTPUT_DIR}/"
ls -lh ${OUTPUT_DIR}/
echo ""
echo "📤 Ready to upload to GitHub Releases!"
