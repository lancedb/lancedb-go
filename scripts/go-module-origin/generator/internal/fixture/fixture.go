// Package fixture builds minimal binary headers for generator tests.
//
// The files are not runnable libraries; they carry only the class, type,
// machine and bitness fields the generator's format inspection reads.
package fixture

import (
	"encoding/binary"
	"fmt"
)

// ELF returns a minimal 64-bit little-endian shared object for the machine.
func ELF(arch string) []byte {
	var machine uint16
	switch arch {
	case "amd64":
		machine = 0x3E
	case "arm64":
		machine = 0xB7
	default:
		panic(fmt.Sprintf("fixture: unsupported ELF arch %q", arch))
	}
	data := make([]byte, 64)
	copy(data, []byte{0x7F, 'E', 'L', 'F', 2, 1, 1, 0})
	binary.LittleEndian.PutUint16(data[16:], 3) // ET_DYN
	binary.LittleEndian.PutUint16(data[18:], machine)
	binary.LittleEndian.PutUint32(data[20:], 1) // EV_CURRENT
	return data
}

// MachO returns a minimal thin 64-bit arm64 dylib.
func MachO() []byte {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint32(data[0:], 0xFEEDFACF)
	binary.LittleEndian.PutUint32(data[4:], 0x0100000C) // CPU_TYPE_ARM64
	binary.LittleEndian.PutUint32(data[8:], 0)          // CPU_SUBTYPE_ARM64_ALL
	binary.LittleEndian.PutUint32(data[12:], 6)         // MH_DYLIB
	return data
}

// PE returns a minimal 64-bit DLL for the machine.
func PE(arch string) []byte {
	var machine uint16
	switch arch {
	case "amd64":
		machine = 0x8664
	case "arm64":
		machine = 0xAA64
	default:
		panic(fmt.Sprintf("fixture: unsupported PE arch %q", arch))
	}
	const headerOffset = 0x80
	const optionalSize = 0xF0
	data := make([]byte, headerOffset+4+20+optionalSize)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3C:], headerOffset)
	copy(data[headerOffset:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[headerOffset+4:], machine)
	binary.LittleEndian.PutUint16(data[headerOffset+20:], optionalSize)
	binary.LittleEndian.PutUint16(data[headerOffset+22:], 0x2000) // IMAGE_FILE_DLL
	binary.LittleEndian.PutUint16(data[headerOffset+24:], 0x20B)  // PE32+
	// NumberOfRvaAndSizes must match the declared optional-header size.
	binary.LittleEndian.PutUint32(data[headerOffset+24+108:], 16)
	return data
}
