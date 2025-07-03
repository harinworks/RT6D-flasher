package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"go.bug.st/serial"
)

type SPIFlash struct {
	port serial.Port
}

const (
	CHUNK_SIZE     = 1024
	SPI_FLASH_SIZE = 4 * 1024 * 1024 // 4MB
)

func NewSPIFlash() *SPIFlash {
	return &SPIFlash{}
}

func (s *SPIFlash) calculateChecksum(command []byte) byte {
	var sum byte = 0
	for _, b := range command {
		sum += b
	}
	return sum + 82
}

func (s *SPIFlash) verify(command []byte) bool {
	if len(command) < 1 {
		return false
	}
	
	lastIdx := len(command) - 1
	var calculatedSum byte = 0
	
	for _, b := range command[:lastIdx] {
		calculatedSum += b
	}
	
	return command[lastIdx] == calculatedSum
}

func (s *SPIFlash) printHex(data []byte) {
	for _, b := range data {
		fmt.Printf("%02X ", b)
	}
	fmt.Println()
}

func (s *SPIFlash) connectToPort(portName string) error {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	
	port, err := serial.Open(portName, mode)
	if err != nil {
		return fmt.Errorf("failed to open port %s: %v", portName, err)
	}
	
	s.port = port
	
	// Set read timeout to 2 seconds like in the Rust code
	return nil
}

// Implementación exacta del código Rust command_readspiflash
func (s *SPIFlash) commandReadSPIFlash(offset uint32) ([]byte, error) {
	command := make([]byte, 4)
	command[0] = 0x52
	command[1] = byte((offset >> 8) & 0xFF)  // Solo usamos los 2 bytes menos significativos
	command[2] = byte(offset & 0xFF)
	command[3] = s.calculateChecksum(command[:3]) // Checksum de los primeros 3 bytes
	
	fmt.Printf("TX (readspiflash): ")
	s.printHex(command)
	
	_, err := s.port.Write(command)
	if err != nil {
		return nil, err
	}
	
	// Leer primer bloque
	block := make([]byte, 1028)
	_, err = s.port.Read(block)
	if err != nil {
		return nil, err
	}
	
	fmt.Printf("RX (readspiflash, bloque 1): ")
	s.printHex(block[:16])
	fmt.Println("...")
	
	// Si no pasa la verificación, leer segundo bloque
	if !s.verify(block) {
		_, err = s.port.Read(block)
		if err != nil {
			return nil, err
		}
		fmt.Printf("RX (readspiflash, bloque 2): ")
		s.printHex(block[:16])
		fmt.Println("...")
	}
	
	if s.verify(block) {
		data := make([]byte, 1024)
		copy(data, block[3:1027])
		return data, nil
	}
	
	return nil, fmt.Errorf("verification failed")
}

func (s *SPIFlash) dumpSPIFlash(filename string) error {
	fmt.Println("Starting SPI flash dump...")
	
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()
	
	for offset := uint32(0); offset < 4096; offset++ { // 4MB / 1024 = 4096 iteraciones
		maxRetries := 3
		var data []byte
		
		for retries := 0; retries < maxRetries; retries++ {
			result, err := s.commandReadSPIFlash(offset)
			if err == nil {
				fmt.Printf("\rDumping SPI flash from address %#08x", offset*1024)
				data = result
				break
			}
			
			if retries < maxRetries-1 {
				fmt.Printf("\rTimeout at %#08x, retrying (%d/%d)", offset*1024, retries+1, maxRetries)
				time.Sleep(100 * time.Millisecond)
			} else {
				return fmt.Errorf("failed after %d retries at offset %#08x: %v", maxRetries, offset*1024, err)
			}
		}
		
		if data != nil {
			_, err := file.Write(data)
			if err != nil {
				return fmt.Errorf("failed to write to file: %v", err)
			}
		}
		
		// Progress indication
		if (offset+1)%100 == 0 {
			progress := float64(offset+1) / 4096.0 * 100
			fmt.Printf(" (%.1f%%)", progress)
		}
	}
	
	fmt.Printf("\nSPI flash dump complete: %s\n", filename)
	return nil
}

func (s *SPIFlash) getAvailablePorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil {
		return []string{}
	}
	sort.Strings(ports)
	return ports
}

func (s *SPIFlash) disconnect() {
	if s.port != nil {
		s.port.Close()
		s.port = nil
	}
}

func showUsage() {
	fmt.Printf("Usage: %s <port> <backup_file>\n", os.Args[0])
	fmt.Println("\nArguments:")
	fmt.Println("  port        Serial port (e.g., /dev/ttyUSB0, COM3)")
	fmt.Println("  backup_file Output file for SPI flash backup")
	fmt.Println("\nExamples:")
	fmt.Printf("  %s /dev/cu.wchusbserial112410 spi_backup.bin\n", os.Args[0])
	fmt.Printf("  %s COM3 spi_backup.bin\n", os.Args[0])
	fmt.Println("\nAvailable serial ports:")
	
	flasher := NewSPIFlash()
	ports := flasher.getAvailablePorts()
	for _, port := range ports {
		fmt.Printf("  %s\n", port)
	}
}

func main() {
	if len(os.Args) != 3 {
		showUsage()
		os.Exit(1)
	}
	
	portName := os.Args[1]
	filename := os.Args[2]
	
	// Verify port exists
	flasher := NewSPIFlash()
	ports := flasher.getAvailablePorts()
	portFound := false
	for _, port := range ports {
		if port == portName {
			portFound = true
			break
		}
	}
	
	if !portFound {
		fmt.Printf("Error: Port '%s' not found\n\n", portName)
		showUsage()
		os.Exit(1)
	}
	
	// Connect to port
	err := flasher.connectToPort(portName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer flasher.disconnect()
	
	fmt.Printf("Connected to port: %s\n", portName)
	fmt.Printf("Output file: %s\n", filename)
	fmt.Println()
	
	fmt.Println("Instructions:")
	fmt.Println("1. Make sure the radio is turned ON")
	fmt.Println("2. Radio should be in NORMAL mode (not programming mode)")
	fmt.Println("3. Press Enter to start SPI flash dump...")
	
	var input string
	fmt.Scanln(&input)
	
	err = flasher.dumpSPIFlash(filename)
	if err != nil {
		fmt.Printf("Backup failed: %v\n", err)
		fmt.Println("\nTroubleshooting:")
		fmt.Println("- Ensure radio is ON and in normal mode")
		fmt.Println("- Check cable connection")
		fmt.Println("- Try turning radio off and on again")
		os.Exit(1)
	}
	
	fmt.Println("SPI flash dump completed successfully!")
}