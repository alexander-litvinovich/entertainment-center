package core

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAddress accepts HDMI1..HDMI4 or a CEC physical address such as 2.1.0.0.
func ParseAddress(value string) (uint16, string, error) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if len(upper) == 5 && strings.HasPrefix(upper, "HDMI") {
		n := upper[4] - '0'
		if n >= 1 && n <= 4 {
			address := uint16(n) << 12
			return address, FormatAddress(address), nil
		}
	}
	parts := strings.Split(upper, ".")
	if len(parts) != 4 {
		return 0, "", fmt.Errorf("input must be HDMI1..HDMI4 or four hex nibbles, for example 2.1.0.0")
	}
	var address uint16
	zeroSeen := false
	for _, part := range parts {
		if len(part) != 1 {
			return 0, "", fmt.Errorf("each physical-address component must be one hex digit")
		}
		n, err := strconv.ParseUint(part, 16, 4)
		if err != nil {
			return 0, "", fmt.Errorf("invalid physical address %q", value)
		}
		if zeroSeen && n != 0 {
			return 0, "", fmt.Errorf("after the first zero all physical-address components must be zero")
		}
		zeroSeen = zeroSeen || n == 0
		address = address<<4 | uint16(n)
	}
	if address == 0 {
		return 0, "", fmt.Errorf("0.0.0.0 is the TV address, not an input")
	}
	return address, FormatAddress(address), nil
}

func FormatAddress(address uint16) string {
	return fmt.Sprintf("%X.%X.%X.%X", address>>12, address>>8&15, address>>4&15, address&15)
}
