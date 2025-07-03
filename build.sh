#!/bin/bash

# RT880 Radio Flasher - Cross-platform build script
# Generates binaries for Linux, Windows, and macOS (both architectures)

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build information
VERSION=$(date +"%Y%m%d-%H%M%S")
BUILD_DIR="dist"

echo -e "${BLUE}RT880 Radio Flasher - Cross-platform Build Script${NC}"
echo -e "${BLUE}=================================================${NC}"
echo "Build version: $VERSION"
echo ""

# Create build directory
echo -e "${YELLOW}Creating build directory...${NC}"
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
go mod download

# Define platforms and architectures
declare -a PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Build function
build_binary() {
    local source_file=$1
    local binary_name=$2
    local goos=$3
    local goarch=$4
    local output_suffix=$5
    
    echo -e "${BLUE}Building ${binary_name} for ${goos}/${goarch}...${NC}"
    
    # Set environment variables
    export GOOS=$goos
    export GOARCH=$goarch
    export CGO_ENABLED=0
    
    # Determine output filename
    local output_name="${binary_name}-${goos}-${goarch}${output_suffix}"
    if [ "$goos" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    # Build the binary
    go build -ldflags="-s -w" -o "${BUILD_DIR}/${output_name}" "$source_file"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Successfully built: ${output_name}${NC}"
        
        # Get file size
        local size=$(ls -lh "${BUILD_DIR}/${output_name}" | awk '{print $5}')
        echo -e "  Size: ${size}"
    else
        echo -e "${RED}✗ Failed to build: ${output_name}${NC}"
        return 1
    fi
    
    echo ""
}

# Build rt880-flasher for all platforms
echo -e "${YELLOW}Building rt880-flasher...${NC}"
echo "=========================="
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r goos goarch <<< "$platform"
    build_binary "main.go" "rt880-flasher" "$goos" "$goarch" ""
done

# Build hex2bin for all platforms
echo -e "${YELLOW}Building hex2bin converter...${NC}"
echo "============================="
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r goos goarch <<< "$platform"
    build_binary "hex2bin.go" "hex2bin" "$goos" "$goarch" ""
done

# Build spi-tool for all platforms
echo -e "${YELLOW}Building spi-tool...${NC}"
echo "===================="
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r goos goarch <<< "$platform"
    build_binary "spi-tool.go" "spi-tool" "$goos" "$goarch" ""
done

# Reset environment variables
unset GOOS
unset GOARCH
unset CGO_ENABLED

# Create release archives
echo -e "${YELLOW}Creating release archives...${NC}"
cd $BUILD_DIR

# Function to create archives
create_archive() {
    local platform=$1
    local archive_name="rt880-flasher-${platform}-${VERSION}"
    
    echo -e "${BLUE}Creating archive for ${platform}...${NC}"
    
    if [ "$platform" = "windows-amd64" ] || [ "$platform" = "windows-arm64" ]; then
        # Windows - create ZIP
        zip -q "${archive_name}.zip" rt880-flasher-${platform}.exe hex2bin-${platform}.exe spi-tool-${platform}.exe
        echo -e "${GREEN}✓ Created: ${archive_name}.zip${NC}"
    else
        # Unix-like - create tar.gz
        tar -czf "${archive_name}.tar.gz" rt880-flasher-${platform} hex2bin-${platform} spi-tool-${platform}
        echo -e "${GREEN}✓ Created: ${archive_name}.tar.gz${NC}"
    fi
}

# Create archives for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r goos goarch <<< "$platform"
    platform_name="${goos}-${goarch}"
    create_archive "$platform_name"
done

cd ..

# Generate checksums
echo -e "${YELLOW}Generating checksums...${NC}"
cd $BUILD_DIR
sha256sum * > checksums.sha256
echo -e "${GREEN}✓ Generated checksums.sha256${NC}"
cd ..

# Display build summary
echo ""
echo -e "${GREEN}Build Summary${NC}"
echo -e "${GREEN}=============${NC}"
echo "Build directory: $BUILD_DIR"
echo "Total files built:"
ls -la $BUILD_DIR | grep -E '\.(exe|tar\.gz|zip)$' | wc -l | xargs echo -e "${GREEN}  Executables and archives:${NC}"
echo ""

echo -e "${BLUE}Files created:${NC}"
ls -la $BUILD_DIR

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
echo ""
echo -e "${YELLOW}Usage examples:${NC}"
echo "  Linux:   ./dist/rt880-flasher-linux-amd64 /dev/ttyUSB0 firmware.hex"
echo "  Windows: ./dist/rt880-flasher-windows-amd64.exe COM3 firmware.hex"
echo "  macOS:   ./dist/rt880-flasher-darwin-arm64 /dev/cu.usbserial firmware.hex"
echo ""
echo -e "${YELLOW}Converter examples:${NC}"
echo "  Linux:   ./dist/hex2bin-linux-amd64 input.hex output.bin"
echo "  Windows: ./dist/hex2bin-windows-amd64.exe input.hex output.bin"
echo "  macOS:   ./dist/hex2bin-darwin-arm64 input.hex output.bin"
echo ""
echo -e "${YELLOW}SPI Tool examples:${NC}"
echo "  Linux:   ./dist/spi-tool-linux-amd64 backup /dev/ttyUSB0 backup.bin"
echo "  Windows: ./dist/spi-tool-windows-amd64.exe restore COM3 backup.bin"
echo "  macOS:   ./dist/spi-tool-darwin-arm64 backup /dev/cu.usbserial backup.bin"