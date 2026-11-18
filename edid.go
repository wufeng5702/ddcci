//go:build windows

package ddcci

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// EDIDInfo is the parsed content of a monitor's Extended Display Identification Data.
type EDIDInfo struct {
	// Raw is the full 128-byte EDID blob as stored in the registry.
	Raw []byte

	// ManufacturerID is the three-letter PnP ID encoded in the EDID header.
	ManufacturerID string

	// ProductCode is the manufacturer-assigned product code.
	ProductCode uint16

	// Serial is the binary serial number.
	Serial uint32

	// Week/Year are the manufacture date.
	Week uint8
	Year int

	// ModelName is the monitor name from the EDID string descriptor, if present.
	ModelName string
}

// readEDIDFromRegistry reads and parses the EDID blob stored under
// SYSTEM\CurrentControlSet\Enum\<class>\<hwid>\<instance>\Device Parameters\EDID.
func readEDIDFromRegistry(keyPath string) (*EDIDInfo, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open registry key %q: %w", keyPath, err)
	}
	defer k.Close()

	raw, _, err := k.GetBinaryValue("EDID")
	if err != nil {
		return nil, fmt.Errorf("read EDID value: %w", err)
	}
	if len(raw) < 128 {
		return nil, fmt.Errorf("EDID too short (%d bytes)", len(raw))
	}

	return parseEDID(raw), nil
}

func parseEDID(raw []byte) *EDIDInfo {
	e := &EDIDInfo{Raw: raw}

	// Manufacturer ID: 15 bits big-endian, 5 bits per letter.
	id := binary.BigEndian.Uint16(raw[8:10])
	e.ManufacturerID = string([]byte{
		byte('A' + (id>>10)&0x1F - 1),
		byte('A' + (id>>5)&0x1F - 1),
		byte('A' + id&0x1F - 1),
	})
	e.ProductCode = binary.LittleEndian.Uint16(raw[10:12])
	e.Serial = binary.LittleEndian.Uint32(raw[12:16])
	e.Week = raw[16]
	e.Year = int(raw[17]) + 1990
	e.ModelName = findEDIDStringDescriptor(raw, 0xFC)
	return e
}

// findEDIDStringDescriptor locates the first string descriptor with the given tag
// (e.g. 0xFC = model name, 0xFF = serial string) in the four descriptor blocks
// at offsets 54, 72, 90, 108.
func findEDIDStringDescriptor(raw []byte, tag byte) string {
	for offset := 54; offset+18 <= 128; offset += 18 {
		if name, ok := parseEDIDDescriptor(raw[offset:offset+18], tag); ok {
			return name
		}
	}
	return ""
}

// isEDIDStringDescriptorHeader reports whether b is a string descriptor block
// of the given tag: 00 00 00 <tag> 00 <text...>.
func isEDIDStringDescriptorHeader(b []byte, tag byte) bool {
	return b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == tag && b[4] == 0
}

// parseEDIDDescriptor decodes an EDID string descriptor block of 18 bytes.
func parseEDIDDescriptor(b []byte, tag byte) (string, bool) {
	if !isEDIDStringDescriptorHeader(b, tag) {
		return "", false
	}

	// Text is padded with 0x0A, then spaces/nulls.
	var out []byte
	for _, c := range b[5:] {
		if c == 0x0A {
			break
		}
		out = append(out, c)
	}

	// Trim trailing spaces/nulls.
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == 0) {
		out = out[:len(out)-1]
	}
	return string(out), true
}
