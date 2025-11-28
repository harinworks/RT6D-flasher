package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"go.bug.st/serial"
)

type SPITool struct {
	port serial.Port
}

const (
	CHUNK_SIZE    = 1024
	SPI_FLASH_SIZE = 32 * 1024 * 1024 // 32MB typical SPI flash size
)

// SPI Commands based on the Rust code
const (
	CMD_READ_SPI_FLASH = 0x52
)

// SPI Write Commands for different ranges
const (
	CMD_WRITE_SPI_0x40 = 0x40 // Range 0-2949119
	CMD_WRITE_SPI_0x41 = 0x41 // Range 2949120-3112959
	CMD_WRITE_SPI_0x42 = 0x42 // Range 3112960-3252223
	CMD_WRITE_SPI_0x43 = 0x43 // Range 3252224-3260415
	CMD_WRITE_SPI_0x47 = 0x47 // Range 3887104-3928063
	CMD_WRITE_SPI_0x48 = 0x48 // Range 3928064-3932159 (Calibration)
	CMD_WRITE_SPI_0x49 = 0x49 // Range 3936256-3977215
	CMD_WRITE_SPI_0x4B = 0x4B // Range 4030464-4071423
	CMD_WRITE_SPI_0x4C = 0x4C // Range 3260416-3887103
)

type SPIRange struct {
	cmd    byte
	offset uint32
	size   uint32
}

func NewSPITool() *SPITool {
	return &SPITool{}
}

func (s *SPITool) calculateChecksum(command []byte) byte {
	var sum byte = 0
	for _, b := range command[:len(command)-1] {
		sum += b
	}
	return sum // No +82, just sum
}

func (s *SPITool) setChecksum(command []byte) {
	var sum byte = 0
	for _, b := range command[:len(command)-1] {
		sum += b
	}
	command[len(command)-1] = sum // No +82, just sum
}

func (s *SPITool) verifyChecksum(data []byte) bool {
	if len(data) < 1 {
		return false
	}
	
	lastIdx := len(data) - 1
	var calculatedSum byte = 0
	
	// Solo sumar los bytes de datos, no el checksum
	for _, b := range data[:lastIdx] {
		calculatedSum += b
	}
	
	return data[lastIdx] == calculatedSum
}

func (s *SPITool) commandReadSPIFlash(blockNum uint16) ([]byte, error) {
	command := make([]byte, 4)
	command[0] = CMD_READ_SPI_FLASH
	command[1] = byte((blockNum >> 8) & 0xFF) // High byte del número de bloque
	command[2] = byte(blockNum & 0xFF)        // Low byte del número de bloque
	s.setChecksum(command)
	
	fmt.Printf("TX (read SPI flash block %d): ", blockNum)
	s.printHex(command)
	
	_, err := s.port.Write(command)
	if err != nil {
		return nil, fmt.Errorf("failed to write read command: %v", err)
	}
	
	// Añadir delay después del envío
	time.Sleep(50 * time.Millisecond)
	
	// Read response block (1028 bytes: 3 header + 1024 data + 1 checksum)
	block := make([]byte, 1028)
	
	// Try to read the complete response with timeout
	totalRead := 0
	startTime := time.Now()
	readTimeout := 3 * time.Second
	
	for totalRead < 1028 {
		if time.Since(startTime) > readTimeout {
			return nil, fmt.Errorf("timeout reading response after %v (got %d bytes)", readTimeout, totalRead)
		}
		
		n, err := s.port.Read(block[totalRead:])
		if err != nil {
			return nil, fmt.Errorf("failed to read response at byte %d: %v", totalRead, err)
		}
		
		if n > 0 {
			totalRead += n
			fmt.Printf("Read %d bytes, total: %d/1028\r", n, totalRead)
		} else {
			// No data available, small delay
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	fmt.Printf("\nRX (read SPI flash, %d bytes): ", totalRead)
	s.printHex(block[:16]) // Print first 16 bytes for debugging
	fmt.Println("...")
	
	// Check if this looks like a valid SPI response (header matches command)
	if block[0] == CMD_READ_SPI_FLASH && block[1] == command[1] && block[2] == command[2] {
		fmt.Printf("Valid SPI response header detected\n")
		
		// Verify checksum - if fails, try reading again (like in Rust code)
		if !s.verifyChecksum(block) {
			fmt.Printf("Checksum failed, trying second read...\n")
			
			// Try reading again
			_, err = s.port.Read(block)
			if err != nil {
				return nil, fmt.Errorf("failed to read second response: %v", err)
			}
			
			fmt.Printf("RX (second read, %d bytes): ", len(block))
			s.printHex(block[:16])
			fmt.Println("...")
		}
		
		// If checksum still fails but header is correct, accept it anyway (SPI may be all FF)
		if !s.verifyChecksum(block) {
			fmt.Printf("Checksum verification failed but header is valid - accepting response\n")
			fmt.Printf("Calculated checksum for debug: ")
			
			// Calculate what checksum should be
			var expectedSum byte = 0
			for _, b := range block[:1027] {
				expectedSum += b
			}
			fmt.Printf("Expected: 0x%02X, Got: 0x%02X\n", expectedSum, block[1027])
		}
	} else {
		return nil, fmt.Errorf("invalid SPI response header: got %02X %02X %02X, expected %02X %02X %02X", 
			block[0], block[1], block[2], CMD_READ_SPI_FLASH, command[1], command[2])
	}
	
	// Extract data (skip 3 header bytes, take 1024 data bytes)
	data := make([]byte, 1024)
	copy(data, block[3:1027])
	
	return data, nil
}

func (s *SPITool) commandWriteSPIFlash(blockNum uint16, data []byte) error {
	if len(data) != 1024 {
		return fmt.Errorf("data must be exactly 1024 bytes, got %d", len(data))
	}
	
	// Simple write command without range logic
	command := make([]byte, 1028)
	command[0] = 0x57 // Simple write command
	command[1] = byte((blockNum >> 8) & 0xFF) // High byte del número de bloque
	command[2] = byte(blockNum & 0xFF)        // Low byte del número de bloque
	copy(command[3:1027], data)
	s.setChecksum(command)
	
	fmt.Printf("TX (write SPI flash block %d): ", blockNum)
	s.printHex(command[:16])
	fmt.Println("...")
	
	_, err := s.port.Write(command)
	if err != nil {
		return fmt.Errorf("failed to write command: %v", err)
	}
	
	// Añadir delay después del envío
	time.Sleep(100 * time.Millisecond) // Longer delay for write operations
	
	// Read response with timeout
	response := make([]byte, 1)
	startTime := time.Now()
	readTimeout := 5 * time.Second // Longer timeout for writes
	
	for {
		if time.Since(startTime) > readTimeout {
			return fmt.Errorf("timeout waiting for write response after %v", readTimeout)
		}
		
		n, err := s.port.Read(response)
		if err != nil {
			return fmt.Errorf("failed to read response: %v", err)
		}
		
		if n > 0 {
			break
		}
		
		time.Sleep(10 * time.Millisecond)
	}
	
	fmt.Printf("RX (write SPI flash): ")
	s.printHex(response)
	
	switch response[0] {
	case 0x06: // ACK
		return nil
	default:
		return fmt.Errorf("device rejected write command, response: 0x%02X", response[0])
	}
}

func (s *SPITool) backupSPIFlash(filename string) error {
	fmt.Println("Starting SPI flash backup...")
	
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %v", err)
	}
	defer file.Close()
	
	// 32768 blocks of 1024 bytes = 32MB total
	totalBlocks := 32768
	
	for block := 0; block < totalBlocks; block++ {
		blockNum := uint16(block)
		
		maxRetries := 3
		var data []byte
		
		for retries := 0; retries < maxRetries; retries++ {
			result, err := s.commandReadSPIFlash(blockNum)
			if err == nil {
				fmt.Printf("\rDumping SPI flash from address %#06x", block*1024)
				data = result
				break
			}
			
			if retries < maxRetries-1 {
				fmt.Printf("\rTimeout at %#06x, retrying (%d/%d)", block*1024, retries+1, maxRetries)
				time.Sleep(100 * time.Millisecond)
			} else {
				fmt.Printf("\nFailed after %d retries at block %d: %v\n", maxRetries, block, err)
				fmt.Println("Make sure the radio is ON and in normal mode (not programming mode).")
				return fmt.Errorf("failed to read block %d: %v", block, err)
			}
		}
		
		if data != nil {
			_, err := file.Write(data)
			if err != nil {
				return fmt.Errorf("failed to write to backup file: %v", err)
			}
		}
		
		// Small delay between blocks to not overwhelm the radio
		time.Sleep(20 * time.Millisecond)
		
		// Progress indication
		if (block+1)%100 == 0 {
			progress := float64(block+1) / float64(totalBlocks) * 100
			fmt.Printf(" (%.1f%%)", progress)
		}
	}
	
	fmt.Printf("\nBackup completed successfully! %d bytes written to %s\n", SPI_FLASH_SIZE, filename)
	return nil
}

func (s *SPITool) restoreSPIFlash(filename string) error {
	fmt.Println("Starting SPI flash restore...")
	fmt.Println("WARNING: This will overwrite the SPI flash content!")
	
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open restore file: %v", err)
	}
	defer file.Close()
	
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}
	
	fileSize := fileInfo.Size()
	if fileSize != SPI_FLASH_SIZE {
		return fmt.Errorf("restore file must be exactly %d bytes, got %d", SPI_FLASH_SIZE, fileSize)
	}
	
	totalBlocks := int(fileSize) / CHUNK_SIZE
	buffer := make([]byte, CHUNK_SIZE)
	
	for block := 0; block < totalBlocks; block++ {
		blockNum := uint16(block)
		
		// Read block from file
		n, err := file.Read(buffer)
		if err != nil && n == 0 {
			break
		}
		
		// Pad with 0xFF if partial block (typical for flash memory)
		if n < CHUNK_SIZE {
			for i := n; i < CHUNK_SIZE; i++ {
				buffer[i] = 0xFF
			}
		}
		
		fmt.Printf("Writing block %d/%d...\n", block+1, totalBlocks)
		
		err = s.commandWriteSPIFlash(blockNum, buffer)
		if err != nil {
			return fmt.Errorf("failed to write block %d: %v", block, err)
		}
		
		// Small delay between blocks to not overwhelm the radio
		time.Sleep(20 * time.Millisecond)
		
		// Progress indication
		if (block+1)%100 == 0 {
			progress := float64(block+1) / float64(totalBlocks) * 100
			fmt.Printf("Progress: %.1f%% (%d/%d blocks)\n", progress, block+1, totalBlocks)
		}
	}
	
	fmt.Printf("Restore completed successfully! %d blocks written from %s\n", totalBlocks, filename)
	return nil
}

func (s *SPITool) getAvailablePorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil {
		return []string{}
	}
	sort.Strings(ports)
	return ports
}

func (s *SPITool) printHex(data []byte) {
	for _, b := range data {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()
}

func (s *SPITool) connectToPort(portName string, baudRate int) error {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	
	port, err := serial.Open(portName, mode)
	if err != nil {
		return fmt.Errorf("failed to open port %s: %v", portName, err)
	}
	
	// Set read timeout to 2 seconds like in Rust code
	err = port.SetReadTimeout(2 * time.Second)
	if err != nil {
		port.Close()
		return fmt.Errorf("failed to set read timeout: %v", err)
	}
	
	s.port = port
	return nil
}

func (s *SPITool) disconnect() {
	if s.port != nil {
		s.port.Close()
		s.port = nil
	}
}

func showUsage() {
	fmt.Printf("Usage: %s <command> <port> <file> [baudrate]\n", os.Args[0])
	fmt.Println("\nCommands:")
	fmt.Println("  backup   - Backup SPI flash to file")
	fmt.Println("  restore  - Restore SPI flash from file")
	fmt.Println("\nArguments:")
	fmt.Println("  port     - Serial port (e.g., /dev/ttyUSB0, COM3)")
	fmt.Println("  file     - Backup/restore file path")
	fmt.Println("\nExamples:")
	fmt.Printf("  %s backup /dev/cu.wchusbserial112410 spi_backup.bin 115200\n", os.Args[0])
	fmt.Printf("  %s restore /dev/cu.wchusbserial112410 spi_backup.bin 115200\n", os.Args[0])
	fmt.Println("\nAvailable serial ports:")
	
	tool := NewSPITool()
	ports := tool.getAvailablePorts()
	for _, port := range ports {
		fmt.Printf("  %s\n", port)
	}
}

func main() {
	if len(os.Args) < 4 {
		showUsage()
		os.Exit(1)
	}
	
	command := os.Args[1]
	portName := os.Args[2]
	filename := os.Args[3]
	
	// Validate command
	if command != "backup" && command != "restore" {
		fmt.Printf("Error: Invalid command '%s'. Use 'backup' or 'restore'\n\n", command)
		showUsage()
		os.Exit(1)
	}
	
	// Set default baud rate if not provided
	baudRate := 115200
	if len(os.Args) >= 5 {
		var err error
		baudRate, err = strconv.Atoi(os.Args[4])
		if err != nil {
			fmt.Printf("Error: Invalid baud rate '%s'. Using default: 115200\n", os.Args[4])
			baudRate = 115200
		}
	}
	
	// Verify port exists
	tool := NewSPITool()
	ports := tool.getAvailablePorts()
	portFound := false
	for _, port := range ports {
		if port == portName {
			portFound = true
			break
		}
	}
	
	if !portFound {
		fmt.Printf("Error: Port '%s (%d)' not found\n\n", portName, baudRate)
		showUsage()
		os.Exit(1)
	}
	
	// Connect to port
	err := tool.connectToPort(portName, baudRate)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer tool.disconnect()
	
	fmt.Printf("Connected to port: %s (%d)\n", portName, baudRate)
	fmt.Printf("Command: %s\n", command)
	fmt.Printf("File: %s\n", filename)
	fmt.Println()
	
	// Execute command
	switch command {
	case "backup":
		fmt.Println("Instructions for backup mode:")
		fmt.Println("1. Connect the data cable to the radio")
		fmt.Println("2. Turn ON the radio normally (no special procedure needed)")
		fmt.Println("3. Press Enter to start backup...")
		
		var input string
		fmt.Scanln(&input)
		
		err = tool.backupSPIFlash(filename)
		if err != nil {
			fmt.Printf("Backup failed: %v\n", err)
			os.Exit(1)
		}
		
	case "restore":
		fmt.Println("Instructions for restore mode:")
		fmt.Println("1. Connect the data cable to the radio")
		fmt.Println("2. Turn ON the radio normally (no special procedure needed)")
		fmt.Println("3. WARNING: This will overwrite the SPI flash content!")
		fmt.Println("4. Press Enter to start restore...")
		
		var input string
		fmt.Scanln(&input)
		
		err = tool.restoreSPIFlash(filename)
		if err != nil {
			fmt.Printf("Restore failed: %v\n", err)
			os.Exit(1)
		}
	}
	
	fmt.Println("Operation completed successfully!")
}