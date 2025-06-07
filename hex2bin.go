package main

import (
	"fmt"
	"os"
)

type HexConverter struct {
	hex      []byte
	allcode  string
	cntcode  int
	writestep int
}

func NewHexConverter() *HexConverter {
	return &HexConverter{
		hex: make([]byte, 251904),
	}
}

func (h *HexConverter) charToInt(char rune) int {
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
	fmt.Printf("Invalid character: %c\n", char)
	return 0
}

func (h *HexConverter) stringOperation() bool {
	if h.allcode == "" {
		return false
	}
	
	i := 0
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered in stringOperation: %v\n", r)
		}
	}()

	// Find next ':'
	for h.cntcode+i < len(h.allcode) && (h.allcode[h.cntcode+i] != ':' || i == 0) {
		i++
	}
	
	if h.cntcode+i >= len(h.allcode) {
		return false
	}

	text := h.allcode[h.cntcode : h.cntcode+i]
	h.cntcode += i

	if len(text) < 9 {
		return false
	}

	num := h.charToInt(rune(text[7]))
	num <<= 4
	recordType := num + h.charToInt(rune(text[8]))

	switch recordType {
	case 0: // Data record
		if len(text) < 10 {
			return true
		}
		
		// Parse length (num2)
		num2 := h.charToInt(rune(text[1]))
		num2 <<= 4
		num2 += h.charToInt(rune(text[2]))
		
		// Parse address (num3)
		num3 := h.charToInt(rune(text[3]))
		num3 <<= 4
		num3 += h.charToInt(rune(text[4]))
		num3 <<= 4
		num3 += h.charToInt(rune(text[5]))
		num3 <<= 4
		num3 += h.charToInt(rune(text[6]))
		
		// Process data bytes
		for j := 0; j < num2; j++ {
			if 9+j*2+1 >= len(text) {
				break
			}
			dataByte := h.charToInt(rune(text[9+j*2]))
			dataByte <<= 4
			dataByte += h.charToInt(rune(text[10+j*2]))
			
			// EXACT C# formula: hex[num3 + j - 10240 + (writestep - 1) * 65536]
			addr := num3 + j - 10240 + (h.writestep-1)*65536
			if addr >= 0 && addr < len(h.hex) {
				h.hex[addr] = byte(dataByte)
				if addr < 10 {
					fmt.Printf("Setting hex[%d] = 0x%02X (from addr 0x%04X+%d, writestep=%d)\n", 
						addr, dataByte, num3, j, h.writestep)
				}
			}
		}
		return true
		
	case 1: // End of file record
		return false
		
	case 4: // Extended linear address record
		h.writestep++
		fmt.Printf("Extended address record - writestep now: %d\n", h.writestep)
		return true
		
	default:
		return true
	}
}

func (h *HexConverter) loadAndConvert(inputFile, outputFile string) error {
	// Initialize hex array with 0x00 (like C# original)
	for i := 0; i < 251904; i++ {
		h.hex[i] = 0x00
	}
	h.writestep = 0
	h.cntcode = 0
	
	// Read input file
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("error reading input file: %v", err)
	}
	
	h.allcode = string(content)
	fmt.Printf("Loaded %d characters from %s\n", len(h.allcode), inputFile)
	fmt.Printf("First 100 chars: %s\n", h.allcode[:min(100, len(h.allcode))])
	
	// Process all Intel HEX records
	recordCount := 0
	for h.stringOperation() {
		recordCount++
	}
	
	fmt.Printf("Processed %d Intel HEX records\n", recordCount)
	fmt.Printf("Final writestep: %d\n", h.writestep)
	
	// Show first and last 16 bytes
	fmt.Printf("First 16 bytes: ")
	for i := 0; i < 16; i++ {
		fmt.Printf("%02X ", h.hex[i])
	}
	fmt.Printf("\n")
	
	fmt.Printf("Last 16 bytes: ")
	for i := 251904 - 16; i < 251904; i++ {
		fmt.Printf("%02X ", h.hex[i])
	}
	fmt.Printf("\n")
	
	// Write binary output
	err = os.WriteFile(outputFile, h.hex, 0644)
	if err != nil {
		return fmt.Errorf("error writing output file: %v", err)
	}
	
	fmt.Printf("Successfully wrote %d bytes to %s\n", len(h.hex), outputFile)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <input_hex_file> <output_bin_file>\n", os.Args[0])
		fmt.Println("\nExample:")
		fmt.Printf("  %s allcode.txt firmware_converted.bin\n", os.Args[0])
		os.Exit(1)
	}
	
	inputFile := os.Args[1]
	outputFile := os.Args[2]
	
	converter := NewHexConverter()
	err := converter.loadAndConvert(inputFile, outputFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Conversion completed successfully!")
}