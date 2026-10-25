package core

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// The adapter always sends from logical address 4 (Playback Device 1).
const (
	logicalAddress   = 4
	maxFrameLen      = 16
	deviceTypePlayer = 4
)

func FrameOn() []byte  { return []byte{logicalAddress<<4 | 0, 0x04} } // Image View On -> TV
func FrameOff() []byte { return []byte{logicalAddress<<4 | 0, 0x36} } // Standby -> TV

// FrameReportPhysicalAddress broadcasts that this playback device sits at address.
func FrameReportPhysicalAddress(address uint16) []byte {
	return []byte{logicalAddress<<4 | 0xf, 0x84, byte(address >> 8), byte(address), deviceTypePlayer}
}

// FrameActiveSource broadcasts that the device at address is the active source.
func FrameActiveSource(address uint16) []byte {
	return []byte{logicalAddress<<4 | 0xf, 0x82, byte(address >> 8), byte(address)}
}

// ParseFrame accepts hex bytes such as "4F:82:20:00", "4f 82 20 00" or "4F822000".
func ParseFrame(text string) ([]byte, error) {
	cleaned := strings.NewReplacer(":", "", " ", "", "-", "").Replace(text)
	frame, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("frame must be hex bytes, for example 4F:82:20:00: %w", err)
	}
	if len(frame) == 0 || len(frame) > maxFrameLen {
		return nil, fmt.Errorf("frame must be 1..%d bytes", maxFrameLen)
	}
	if frame[0]>>4 != logicalAddress {
		return nil, fmt.Errorf("first byte must start with %d: this adapter sends from logical address %d", logicalAddress, logicalAddress)
	}
	return frame, nil
}

func FormatFrame(frame []byte) string {
	parts := make([]string, len(frame))
	for i, b := range frame {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}
