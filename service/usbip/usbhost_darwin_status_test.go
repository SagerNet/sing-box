//go:build darwin && cgo

package usbip

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const (
	// From IOKit/usb/USB.h: kIOUSBPipeStalled
	darwinIOReturnPipeStalled int32 = -536854449
	// From IOKit/usb/IOUSBHostFamilyDefinitions.h: kUSBHostReturnPipeStalled
	darwinUSBHostReturnPipeStalled int32 = -536850432
)

func TestDarwinIOReturnToUSBIPStatusMapsStall(t *testing.T) {
	testCases := []int32{
		darwinIOReturnPipeStalled,
		darwinUSBHostReturnPipeStalled,
	}

	for _, status := range testCases {
		require.Equal(t, -int32(unix.EPIPE), darwinIOReturnToUSBIPStatus(status))
	}
}

func TestDarwinUSBIPStatusToCIStatusMapsStall(t *testing.T) {
	require.Equal(t, ciStatusStallError, darwinUSBIPStatusToCIStatus(-int32(unix.EPIPE)))
}
