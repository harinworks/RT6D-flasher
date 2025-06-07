# RT880 Radio Flasher

A toolkit for flashing firmware to RT880 radios via serial connection.

## Description

This project includes two main programs:

1. **rt880-flasher** (`main.go`) - Main flasher for uploading firmware to RT880 radios
2. **hex2bin** (`hex2bin.go`) - Intel HEX to binary file converter

## Building

### Prerequisites

- Go 1.21 or higher
- Dependency: `go.bug.st/serial` (downloaded automatically)

### Compile both binaries

```bash
# Compile the main flasher
go build -o rt880-flasher main.go

# Compile the hex2bin converter
go build -o hex2bin hex2bin.go
```

### Download dependencies

```bash
go mod download
```

## Usage

### RT880-Flasher

```bash
./rt880-flasher <serial_port> <firmware_file>
```

**Examples:**
```bash
# On Linux/macOS
./rt880-flasher /dev/cu.wchusbserial112410 firmware.hex
./rt880-flasher /dev/ttyUSB0 firmware.bin

# On Windows
./rt880-flasher COM3 firmware.hex
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

## Features

### RT880-Flasher
- Automatic detection of available serial ports
- Support for Intel HEX and binary files
- Communication protocol with retries and timeouts
- Checksum verification
- Real-time progress reporting
- Robust error handling

### Hex2Bin
- Accurate Intel HEX to binary conversion
- Support for extended address records
- RT880-specific ARM address mapping
- Format validation

## Project Files

- `main.go` - Main flasher source code
- `hex2bin.go` - Converter source code
- `go.mod` / `go.sum` - Go dependency configuration
- `RT880G-V1_12.HEX` / `RT880G-V1_12.BIN` - Sample firmware files
- `allcode.txt` - Sample Intel HEX file
- `convert.sh` - Auxiliary conversion script

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
GOOS=linux GOARCH=amd64 go build -o rt880-flasher-linux main.go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o rt880-flasher.exe main.go

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o rt880-flasher-macos-intel main.go

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o rt880-flasher-macos-m1 main.go
```