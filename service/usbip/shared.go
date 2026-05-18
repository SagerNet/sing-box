//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/sagernet/sing-box/option"
)

type clientTarget struct {
	fixedBusID string
	match      option.USBIPDeviceMatch
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func closeConnOnContextDone(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func hex8(v uint8) string {
	const hexdigits = "0123456789abcdef"
	return string([]byte{hexdigits[(v>>4)&0xf], hexdigits[v&0xf]})
}

func describeMatch(m option.USBIPDeviceMatch) string {
	var parts []string
	if m.BusID != "" {
		parts = append(parts, "busid="+m.BusID)
	}
	if m.VendorID != 0 {
		parts = append(parts, fmt.Sprintf("vendor_id=0x%04x", uint16(m.VendorID)))
	}
	if m.ProductID != 0 {
		parts = append(parts, fmt.Sprintf("product_id=0x%04x", uint16(m.ProductID)))
	}
	if m.Serial != "" {
		parts = append(parts, "serial="+m.Serial)
	}
	return "{" + strings.Join(parts, ",") + "}"
}
