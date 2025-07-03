package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Flasher struct {
	port         serial.Port
	writestep    int
	step         int
	recvcnt      int
	sendcnt      int
	gWritebytes  int
	sendbuf      []byte
	recvbuf      []byte
	hex          []byte
	flgConnect   bool
	allcode      string
	cntcode      int
	rep          int

	// Retry and timeout logic
	lastPacketTime time.Time
	retryCount     int
	maxRetries     int
	packetTimeout  time.Duration
	waitingForAck  bool

	// Protocol constants
	sendConnect []byte
	sendEnd     []byte
	sendUpdate  []byte
	sendbufRight []byte
	sendbufError []byte
	checksumOffset byte // Different checksum offset for different radio types
}

func NewFlasher(useIRadio bool) *Flasher {
	f := &Flasher{
		sendbuf:       make([]byte, 2052),
		recvbuf:       make([]byte, 29),
		hex:           make([]byte, 251904),
		sendbufRight:  []byte{6},
		sendbufError:  []byte{255},
		allcode:       "", // Will be loaded from file or kept empty as requested
		maxRetries:    3,
		packetTimeout: 3 * time.Second,
	}
	
	if useIRadio {
		// iRadio parameters (current default)
		f.sendConnect = []byte{57, 51, 5, 16, 129}
		f.sendEnd = []byte{57, 51, 5, 238, 95}
		f.sendUpdate = []byte{57, 51, 5, 85, 198}
		f.checksumOffset = 0 // iRadio uses +82 offset
		fmt.Println("Using iRadio protocol parameters")
	} else {
		// Retevis/Radtel parameters (original/older protocol)
		f.sendConnect = []byte{57, 51, 5, 16, 211}
		f.sendEnd = []byte{57, 51, 5, 238, 177}
		f.sendUpdate = []byte{57, 51, 5, 85, 24}
		f.checksumOffset = 82 // Retevis/Radtel uses no additional offset
		fmt.Println("Using Retevis/Radtel protocol parameters")
	}
	
	f.sendbuf[0] = 87
	return f
}

func (f *Flasher) generateCheckCode(codeCount int) string {
	result := ""
	num := time.Now().UnixNano() + int64(f.rep)
	f.rep++
	
	// Simplified random generation
	for i := 0; i < codeCount; i++ {
		num = num*1103515245 + 12345
		if (num%2) != 0 {
			result += string(rune(65 + (num % 26)))
		} else {
			result += string(rune(48 + (num % 10)))
		}
	}
	return result
}

func (f *Flasher) initializeHex(firmwareFile string) bool {
	for i := 0; i < 251904; i++ {
		f.hex[i] = 0xFF // Initialize with 0xFF instead of 0x00 (typical for flash memory)
	}
	f.gWritebytes = 0
	f.writestep = 0
	
	// Load firmware based on file extension
	var loaded bool
	if strings.HasSuffix(strings.ToLower(firmwareFile), ".bin") {
		loaded = f.loadBinaryFirmware(firmwareFile)
		if loaded {
			fmt.Printf("Loaded binary firmware: %s\n", firmwareFile)
		}
	} else if strings.HasSuffix(strings.ToLower(firmwareFile), ".hex") {
		loaded = f.loadStandardIntelHex(firmwareFile)
		if loaded {
			fmt.Printf("Loaded Intel HEX firmware: %s\n", firmwareFile)
		}
	} else {
		// Try to detect format by content
		if f.loadStandardIntelHex(firmwareFile) {
			loaded = true
			fmt.Printf("Loaded Intel HEX firmware: %s\n", firmwareFile)
		} else if f.loadBinaryFirmware(firmwareFile) {
			loaded = true
			fmt.Printf("Loaded binary firmware: %s\n", firmwareFile)
		}
	}
	
	if !loaded {
		fmt.Printf("Failed to load firmware file: %s\n", firmwareFile)
		return false
	}
	
	// Show some hex data for verification
	fmt.Printf("First 16 bytes of hex array: ")
	for i := 0; i < 16; i++ {
		fmt.Printf("%02X ", f.hex[i])
	}
	fmt.Printf("\n")
	return true
}

func (f *Flasher) loadStandardIntelHex(filename string) bool {
	fmt.Printf("Attempting to load standard Intel HEX firmware: %s\n", filename)
	
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Standard Intel HEX file not found: %s\n", filename)
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	recordCount := 0
	extendedAddress := 0
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) <= 1 || !strings.HasPrefix(line, ":") {
			continue // Skip empty lines or lines with just ":"
		}
		
		if !f.processIntelHexRecord(line, &extendedAddress) {
			fmt.Printf("Error processing Intel HEX record: %s\n", line)
			return false
		}
		recordCount++
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading Intel HEX file: %v\n", err)
		return false
	}
	
	fmt.Printf("Processed %d Intel HEX records\n", recordCount)
	return recordCount > 0
}

func (f *Flasher) processIntelHexRecord(record string, extendedAddress *int) bool {
	if len(record) < 11 {
		fmt.Printf("Skipping short record: %s\n", record)
		return true // Skip short records instead of failing
	}
	
	// Parse record length
	length, err := strconv.ParseInt(record[1:3], 16, 32)
	if err != nil {
		return false
	}
	
	// Parse address
	addr, err := strconv.ParseInt(record[3:7], 16, 32)
	if err != nil {
		return false
	}
	
	// Parse record type
	recordType, err := strconv.ParseInt(record[7:9], 16, 32)
	if err != nil {
		return false
	}
	
	switch recordType {
	case 0: // Data record
		fullAddress := *extendedAddress + int(addr)
		// Map ARM addresses to our hex array (subtract base address 0x08002800)
		targetBase := fullAddress - 0x08002800
		if targetBase < 0 {
			return true // Skip records before our target area  
		}
		
		for i := 0; i < int(length); i++ {
			if 9+i*2+1 >= len(record) {
				break
			}
			dataByte, err := strconv.ParseInt(record[9+i*2:11+i*2], 16, 32)
			if err != nil {
				return false
			}
			
			targetAddr := targetBase + i
			if targetAddr >= 0 && targetAddr < len(f.hex) {
				f.hex[targetAddr] = byte(dataByte)
			}
		}
	case 1: // End of file
		return true
	case 4: // Extended Linear Address
		if length != 2 {
			return false
		}
		extAddr, err := strconv.ParseInt(record[9:13], 16, 32)
		if err != nil {
			return false
		}
		*extendedAddress = int(extAddr) << 16
	}
	
	return true
}

func (f *Flasher) loadBinaryFirmware(filename string) bool {
	fmt.Printf("Attempting to load binary firmware: %s\n", filename)
	
	// Check if file exists and get info
	fileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Printf("Binary file not found: %s\n", filename)
		return false
	}
	fmt.Printf("Binary file size: %d bytes\n", fileInfo.Size())
	
	// Read binary file content
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading binary file %s: %v\n", filename, err)
		return false
	}
	
	// Copy binary data directly to hex array
	copySize := min(len(content), len(f.hex))
	copy(f.hex[:copySize], content)
	
	fmt.Printf("Loaded %d bytes of binary firmware\n", copySize)
	return true
}

func (f *Flasher) loadFirmwareFromFile(filename string) bool {
	fmt.Printf("Attempting to load Intel HEX firmware: %s\n", filename)
	
	// Check if file exists and get info
	fileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Printf("Intel HEX file not found: %s\n", filename)
		return false
	}
	fmt.Printf("Intel HEX file size: %d bytes\n", fileInfo.Size())
	
	// Read entire file content
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading Intel HEX file %s: %v\n", filename, err)
		return false
	}
	
	f.allcode = string(content)
	f.cntcode = 0
	fmt.Printf("Loaded Intel HEX data from %s (%d chars)\n", filename, len(f.allcode))
	
	if len(f.allcode) > 0 {
		fmt.Printf("First 100 chars: %s\n", f.allcode[:min(100, len(f.allcode))])
		return true
	} else {
		fmt.Printf("Warning: File was read but content is empty!\n")
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (f *Flasher) checksum(array []byte, length int) byte {
	var sum byte = 0
	for i := 0; i < length-1; i++ {
		sum += array[i]
	}
	return sum + f.checksumOffset
}

func (f *Flasher) charToInt(char rune) int {
	num := int(char)
	if num > 47 && num < 58 {
		return num - 48
	}
	if num > 64 && num < 71 {
		return num - 55
	}
	if num > 96 && num < 103 {
		return num - 87
	}
	fmt.Printf("发现无效字符: %c\n", char)
	return 0
}

func (f *Flasher) stringOperation() bool {
	if f.allcode == "" {
		return false
	}
	
	i := 0
	defer func() {
		if r := recover(); r != nil {
			// Handle panic gracefully
		}
	}()

	// Find next ':'
	for f.cntcode+i < len(f.allcode) && (f.allcode[f.cntcode+i] != ':' || i == 0) {
		i++
	}
	
	if f.cntcode+i >= len(f.allcode) {
		return false
	}

	text := f.allcode[f.cntcode : f.cntcode+i]
	f.cntcode += i

	if len(text) < 9 {
		return false
	}

	num := f.charToInt(rune(text[7]))
	num <<= 4
	recordType := num + f.charToInt(rune(text[8]))

	switch recordType {
	case 0: // Data record
		if len(text) < 10 {
			return true
		}
		
		num2 := f.charToInt(rune(text[1]))
		num2 <<= 4
		num2 += f.charToInt(rune(text[2]))
		
		num3 := f.charToInt(rune(text[3]))
		num3 <<= 4
		num3 += f.charToInt(rune(text[4]))
		num3 <<= 4
		num3 += f.charToInt(rune(text[5]))
		num3 <<= 4
		num3 += f.charToInt(rune(text[6]))
		
		for j := 0; j < num2; j++ {
			if 9+j*2+1 >= len(text) {
				break
			}
			num := f.charToInt(rune(text[9+j*2]))
			num <<= 4
			num += f.charToInt(rune(text[10+j*2]))
			
			addr := num3 + j - 10240 + (f.writestep-1)*65536
			if addr >= 0 && addr < len(f.hex) {
				f.hex[addr] = byte(num)
			}
		}
		return true
	case 1: // End of file record
		return false
	case 4: // Extended linear address record
		f.writestep++
		return true
	default:
		return true
	}
}

func (f *Flasher) getAvailablePorts() []string {
	ports, err := serial.GetPortsList()
	if err != nil {
		return []string{}
	}
	sort.Strings(ports)
	return ports
}

func (f *Flasher) clearRecvbuf() {
	for i := 0; i < len(f.recvbuf); i++ {
		f.recvbuf[i] = 255
	}
	f.recvcnt = 0
}

func (f *Flasher) dataSum(array []byte) byte {
	var sum byte = 0
	for i := 0; i < int(array[2])-1; i++ {
		sum += array[i]
	}
	return sum
}

func (f *Flasher) revDateOperation() {
	if f.recvcnt != 1 {
		return
	}

	fmt.Printf("Processing received byte: 0x%02X in step %d\n", f.recvbuf[0], f.step)

	switch f.recvbuf[0] {
	case 50: // 0x32
		// Do nothing
		break
	case 0: // Connection established
		f.recvcnt = 0
		break
	case 255: // NAK - Error
		if f.step == 4 && f.sendcnt > 0 {
			// NAK during data transfer - retry the packet
			fmt.Printf("NAK received! Block %d rejected. Data at offset %d--%d\n", 
				f.gWritebytes, f.sendcnt-1024, f.sendcnt-1)
			
			// Show first few bytes of the rejected block for debugging
			fmt.Printf("Rejected block data (first 16 bytes): ")
			startOffset := f.sendcnt - 1024
			if startOffset >= 0 {
				for i := 0; i < 16 && startOffset+i < len(f.hex); i++ {
					fmt.Printf("%02X ", f.hex[startOffset+i])
				}
			}
			fmt.Printf("\n")
			
			// Show the checksum that was sent
			fmt.Printf("Sent checksum: 0x%02X\n", f.sendbuf[1027])
			
			// Retry the packet
			f.retryLastPacket()
		} else {
			// NAK during connection phase
			fmt.Printf("NAK received during connection phase (step %d)\n", f.step)
			f.port.Close()
			f.step = 0
			fmt.Println("Communication Error - NAK received!")
			f.clearRecvbuf()
		}
		break
	case 6: // ACK - acknowledgment
		f.recvcnt = 0
		f.waitingForAck = false // Clear waiting state
		f.retryCount = 0        // Reset retry counter
		
		if f.step < 3 {
			f.step++
			fmt.Printf("Connection step %d, sending connect command\n", f.step)
			f.port.Write(f.sendConnect)
			time.Sleep(50 * time.Millisecond)
		} else if f.step == 3 {
			fmt.Println("Sending update command")
			f.port.Write(f.sendUpdate)
			time.Sleep(50 * time.Millisecond)
			f.step = 4
		} else if f.step == 4 {
			// Data transfer phase - ACK received, can send next packet
			fmt.Printf("ACK received for block %d\n", f.gWritebytes)
			
			// Send next packet
			f.gWritebytes++
			fmt.Printf("Progress: %03d/246 (sending block at offset %d)\n", f.gWritebytes, f.sendcnt)
			
			f.sendbuf[1] = byte(f.sendcnt >> 8)
			f.sendbuf[2] = byte(f.sendcnt & 0xFF)
			
			for i := 0; i < 1024; i++ {
				f.sendbuf[3+i] = f.hex[f.sendcnt+i]
			}
			f.sendbuf[1027] = f.checksum(f.sendbuf, 1028)
			
			f.sendDataPacket()
			f.sendcnt += 1024
			
			if f.sendcnt >= 251904 {
				f.step = 5
				fmt.Println("Data transfer completed! Sending end command...")
				f.port.Write(f.sendEnd)
				time.Sleep(100 * time.Millisecond)
				f.port.Close()
			}
		}
		break
	default:
		fmt.Printf("Unknown response: 0x%02X\n", f.recvbuf[0])
		f.recvcnt = 0
		break
	}
}

func (f *Flasher) sendDataPacket() {
	fmt.Printf("Sending block data (first 16 bytes): ")
	for i := 0; i < 16; i++ {
		fmt.Printf("%02X ", f.sendbuf[3+i])
	}
	fmt.Printf("\n")
	fmt.Printf("Block header: %02X %02X %02X, checksum: %02X\n", 
		f.sendbuf[0], f.sendbuf[1], f.sendbuf[2], f.sendbuf[1027])
	
	n, err := f.port.Write(f.sendbuf[:1028])
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
	} else {
		fmt.Printf("Sent %d bytes\n", n)
	}
	
	f.lastPacketTime = time.Now()
	f.waitingForAck = true
	f.retryCount = 0
}

func (f *Flasher) retryLastPacket() {
	if f.retryCount < f.maxRetries {
		f.retryCount++
		fmt.Printf("Timeout! Retrying packet (attempt %d/%d) - going back to block %d\n", 
			f.retryCount, f.maxRetries, f.gWritebytes-1)
		
		// Go back one packet
		f.sendcnt -= 1024
		f.gWritebytes--
		f.waitingForAck = false
		
		fmt.Printf("Reset state: sendcnt=%d, gWritebytes=%d, waitingForAck=%t\n", 
			f.sendcnt, f.gWritebytes, f.waitingForAck)
	} else {
		fmt.Printf("Max retries exceeded. Aborting transfer.\n")
		f.port.Close()
		f.step = 0
	}
}

func (f *Flasher) checkTimeout() {
	if f.waitingForAck && time.Since(f.lastPacketTime) > f.packetTimeout {
		fmt.Printf("Timeout detected! Waiting for ACK for %.1f seconds\n", time.Since(f.lastPacketTime).Seconds())
		f.retryLastPacket()
	}
}

func (f *Flasher) readData() {
	buffer := make([]byte, 1)
	for f.port != nil {
		// Check for timeout on each loop
		f.checkTimeout()
		
		n, err := f.port.Read(buffer)
		if err != nil || n == 0 {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		
		if f.recvcnt < len(f.recvbuf) {
			f.recvbuf[f.recvcnt] = buffer[0]
			f.recvcnt++
			fmt.Printf("Received byte: 0x%02X (step: %d, recvcnt: %d)\n", buffer[0], f.step, f.recvcnt)
			
			if f.recvbuf[0] == 0 {
				f.sendcnt = 0
				f.flgConnect = true
				time.Sleep(200 * time.Millisecond)
			} else {
				f.flgConnect = false
				time.Sleep(1 * time.Millisecond)
				f.revDateOperation()
			}
		}
	}
}

func (f *Flasher) startUpdate(portName string) error {
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
	f.port = port

	f.gWritebytes = 0
	f.step = 1
	f.sendcnt = 0
	f.flgConnect = true

	// Start reading in goroutine
	go f.readData()

	// Initial connection attempts
	fmt.Println("Attempting to connect...")
	f.port.Write(f.sendConnect)
	time.Sleep(200 * time.Millisecond)
	
	if f.flgConnect {
		f.sendcnt = 0
		f.port.Write(f.sendConnect)
		time.Sleep(200 * time.Millisecond)
	}
	
	if f.flgConnect {
		f.sendcnt = 0  
		f.port.Write(f.sendConnect)
		time.Sleep(200 * time.Millisecond)
	}
	
	if f.flgConnect {
		f.sendcnt = 0
		f.port.Write(f.sendConnect)
		time.Sleep(200 * time.Millisecond)
	}

	if f.flgConnect {
		f.port.Close()
		return fmt.Errorf("communication error - no response from device")
	}
	
	fmt.Println("Device connected, starting firmware upload...")
	
	// Keep the connection alive
	for f.step < 5 && f.port != nil {
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func showUsage() {
	fmt.Printf("Usage: %s [options] <port> <firmware_file>\n", os.Args[0])
	fmt.Println("\nArguments:")
	fmt.Println("  port          Serial port (e.g., /dev/ttyUSB0, COM3)")
	fmt.Println("  firmware_file Firmware file (.hex or .bin)")
	fmt.Println("\nOptions:")
	fmt.Println("  -iradio       Use iRadio protocol parameters (for older radio models)")
	fmt.Println("\nExamples:")
	fmt.Printf("  %s /dev/cu.wchusbserial112410 firmware.hex\n", os.Args[0])
	fmt.Printf("  %s -iradio COM3 firmware.bin\n", os.Args[0])
	fmt.Println("\nAvailable serial ports:")
	
	flasher := NewFlasher(false)
	ports := flasher.getAvailablePorts()
	for _, port := range ports {
		fmt.Printf("  %s\n", port)
	}
}

func main() {
	// Parse command line arguments
	useIRadio := false
	var portName, firmwareFile string
	
	args := os.Args[1:] // Remove program name
	
	// Check for -iradio flag
	for i, arg := range args {
		if arg == "-iradio" {
			useIRadio = true
			// Remove the flag from args
			args = append(args[:i], args[i+1:]...)
			break
		}
	}
	
	// Check remaining arguments
	if len(args) != 2 {
		showUsage()
		os.Exit(1)
	}
	
	portName = args[0]
	firmwareFile = args[1]
	
	// Verify port exists
	flasher := NewFlasher(useIRadio)
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
	
	// Load firmware
	if !flasher.initializeHex(firmwareFile) {
		os.Exit(1)
	}

	fmt.Printf("Selected port: %s\n", portName)
	fmt.Printf("Firmware file: %s\n", firmwareFile)

	fmt.Println("\nInstructions:")
	fmt.Println("1. Connect the data cable to the radio")
	fmt.Println("2. Turn OFF the radio completely")
	fmt.Println("3. Press and HOLD the PTT key")
	fmt.Println("4. While holding PTT, turn ON the radio")
	fmt.Println("5. Keep holding PTT for 2-3 seconds after power on")
	fmt.Println("6. Release PTT - radio should be in programming mode")
	fmt.Println("7. Press Enter to start upgrade...")
	
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	err := flasher.startUpdate(portName)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Update completed successfully!")
}