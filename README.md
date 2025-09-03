# RT6D Radio Flasher

A toolkit for flashing firmware to RT6D radios via serial connection.

## Description

This project includes four main programs:

1. **rt6d-flasher** (`main.go`) - Main flasher for uploading firmware to RT6D radios
2. **hex2bin** (`hex2bin.go`) - Intel HEX to binary file converter
3. **spi-tool** (`spi-tool.go`) - SPI flash backup and restore utility
4. **spi-flash** (`spi-flash.go`) - Alternative SPI flash tool

## Building

### Prerequisites

- Go 1.21 or higher
- Dependency: `go.bug.st/serial` (downloaded automatically)

### Compile all binaries

```bash
# Compile the main flasher
go build -o rt6d-flasher main.go

# Compile the hex2bin converter
go build -o hex2bin hex2bin.go

# Compile the SPI tool
go build -o spi-tool spi-tool.go

# Compile the alternative SPI flash tool
go build -o spi-flash spi-flash.go

# Or use the build script
./build.sh
```

### Download dependencies

```bash
go mod download
```

## Usage

### RT6D-Flasher

```bash
./rt6d-flasher <serial_port> <firmware_file> [flags]
```

**Flags:**
- `-iradio` - Use for Iradio UV98 Plus model

**Examples:**
```bash
# On Linux/macOS
./rt6d-flasher /dev/cu.wchusbserial112410 firmware.hex
./rt6d-flasher /dev/ttyUSB0 firmware.bin

# For Iradio UV98 Plus
./rt6d-flasher /dev/cu.wchusbserial112410 firmware.hex -iradio
./rt6d-flasher /dev/ttyUSB0 firmware.bin -iradio

# On Windows
./rt6d-flasher COM3 firmware.hex

# For Iradio UV98 Plus on Windows
./rt6d-flasher COM3 firmware.hex -iradio
```

**Supported firmware formats:**
- Intel HEX (`.hex`)
- Binary (`.bin`)

**Flashing procedure:**
1. Connect the data cable to the radio
2. Turn OFF the radio completely
3. Press and HOLD the PTT key
4. While holding PTT, turn ON the radio
5. Keep holding PTT for 2-3 seconds after power on
6. Release PTT - radio should be in programming mode
7. Press Enter to start the update

### Hex2Bin Converter

```bash
./hex2bin <input_hex_file> <output_bin_file>
```

**Example:**
```bash
./hex2bin allcode.txt firmware_converted.bin
```

### SPI Tool

```bash
./spi-tool <command> <serial_port> <file>
```

**Commands:**
- `backup` - Backup SPI flash to file
- `restore` - Restore SPI flash from file

**Examples:**
```bash
# Backup SPI flash
./spi-tool backup /dev/cu.wchusbserial112410 spi_backup.bin
./spi-tool backup COM3 spi_backup.bin

# Restore SPI flash
./spi-tool restore /dev/ttyUSB0 spi_backup.bin
./spi-tool restore COM3 spi_backup.bin
```

**SPI Tool procedure:**
1. Connect the data cable to the radio
2. Turn ON the radio normally (no special procedure needed)
3. Press Enter to start backup/restore operation

## Features

### RT6D-Flasher
- Automatic detection of available serial ports
- Support for Intel HEX and binary files
- Communication protocol with retries and timeouts
- Checksum verification
- Real-time progress reporting
- Robust error handling

### Hex2Bin
- Accurate Intel HEX to binary conversion
- Support for extended address records
- RT6D-specific ARM address mapping
- Format validation

### SPI Tool
- Complete SPI flash backup (4MB)
- SPI flash restore from backup file
- Block-by-block operation with progress indication
- Checksum verification for data integrity
- Automatic retry mechanism for failed operations

## Project Files

### Source Code
- `main.go` - Main flasher source code
- `hex2bin.go` - Converter source code
- `spi-tool.go` - SPI tool source code
- `spi-flash.go` - Alternative SPI flash tool
- `go.mod` / `go.sum` - Go dependency configuration

### Compiled Binaries
- `rt6d-flasher` - Main flasher executable
- `rt6d-flasher-windows-arm64` - Windows ARM64 build
- `hex2bin` - Converter executable
- `spi-tool` - SPI tool executable
- `spi-flash` - Alternative SPI flash executable

### Firmware Files
- `HEX/` - Intel HEX firmware files directory
  - `RT880-V1_12A.HEX`
  - `RT880G-V1_12.HEX`
- `BIN/` - Binary firmware files directory
  - `RT880-V1_12A.BIN`
  - `RT880G-V1_12.BIN`
- `RT880G_V1.14.bin` - Firmware v1.14 for RT880G
- `RT880_V1.14.bin` - Firmware v1.14 for RT880
- `RT880-V1_12A.cs` - C# source for RT880 v1.12A
- `RT880G-V1_12.cs` - C# source for RT880G v1.12

### Scripts and Tools
- `build.sh` - Build script
- `dist/` - Distribution directory

### Backup Files
- `kk.bin`, `kk.spi` - Sample backup files
- `spi.bin`, `spi2.bin`, `spi3.bin` - SPI flash backup files

## Serial Configuration

- **Baud Rate:** 115200
- **Data Bits:** 8
- **Parity:** None
- **Stop Bits:** 1

## Troubleshooting

1. **Port not found:** Verify the device is connected and drivers are installed
2. **Communication error:** Ensure the radio is in programming mode (follow PTT procedure)
3. **Checksum error:** Verify firmware file integrity
4. **Timeout:** Check cable connection and radio status

## Cross-Platform Compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o rt6d-flasher-linux main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o rt6d-flasher.exe main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o rt6d-flasher-macos-intel main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o rt6d-flasher-macos-m1 main.go
```