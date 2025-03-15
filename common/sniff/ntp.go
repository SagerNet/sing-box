package sniff

import (
	"context"
	"encoding/binary"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
)

func NTP(ctx context.Context, metadata *adapter.InboundContext, packet []byte) error {
	// NTP packets must be at least 48 bytes long (standard NTP header size).
	pLen := len(packet)
	if pLen < 48 {
		return os.ErrInvalid
	}
	// Check the LI (Leap Indicator) and Version Number (VN) in the first byte.
	// We'll primarily focus on ensuring the version is valid for NTP.
	// Many NTP versions are used, but let's check for generally accepted ones (3 & 4 for IPv4, plus potential extensions/customizations)
	firstByte := packet[0]
	li := (firstByte >> 6) & 0x03 // Extract LI
	vn := (firstByte >> 3) & 0x07 // Extract VN
	mode := firstByte & 0x07      // Extract Mode

	// Leap Indicator should be a valid value (0-3).
	if li > 3 {
		return os.ErrInvalid
	}

	// Version Check (common NTP versions are 3 and 4)
	if vn != 3 && vn != 4 {
		return os.ErrInvalid
	}

	// Check the Mode field for a client request (Mode 3).  This validates it *is* a request.
	if mode != 3 {
		return os.ErrInvalid
	}

	// Check Root Delay and Root Dispersion. While not strictly *required* for a request,
	// we can check if they appear to be reasonable values (not excessively large).
	rootDelay := binary.BigEndian.Uint32(packet[4:8])
	rootDispersion := binary.BigEndian.Uint32(packet[8:12])

	// Check for unreasonably large root delay and dispersion.  NTP RFC specifies max values of approximately 16 seconds.
	// Convert to milliseconds for easy comparison.  Each unit is 1/2^16 seconds.
	if float64(rootDelay)/65536.0 > 16.0 {
		return os.ErrInvalid
	}
	if float64(rootDispersion)/65536.0 > 16.0 {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolNTP

	return nil
}
