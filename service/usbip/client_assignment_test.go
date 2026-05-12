//go:build linux || (darwin && cgo)

package usbip

import (
	"sort"
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestClientAssignmentApplyAllEmitsStartForNewDesired(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment(nil)
	entries := []DeviceEntry{standardTestDeviceEntry("1-1"), standardTestDeviceEntry("2-1")}

	start, stop := assignment.ApplyAll(entries)
	sort.Strings(start)
	require.Equal(t, []string{"1-1", "2-1"}, start)
	require.Empty(t, stop)
	require.True(t, assignment.IsRetryDesired("1-1"))
	require.True(t, assignment.IsRetryDesired("2-1"))
}

func TestClientAssignmentApplyAllEmitsStopForUndesiredInactive(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment(nil)
	entries := []DeviceEntry{standardTestDeviceEntry("1-1"), standardTestDeviceEntry("2-1")}
	_, _ = assignment.ApplyAll(entries)

	start, stop := assignment.ApplyAll(nil)
	require.Empty(t, start)
	sort.Strings(stop)
	require.Equal(t, []string{"1-1", "2-1"}, stop)
	require.False(t, assignment.IsRetryDesired("1-1"))
}

func TestClientAssignmentApplyAllKeepsActiveButUndesiredRegistered(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment(nil)
	entries := []DeviceEntry{standardTestDeviceEntry("1-1")}
	_, _ = assignment.ApplyAll(entries)
	assignment.SetActive("1-1", true)

	start, stop := assignment.ApplyAll(nil)
	require.Empty(t, start)
	require.Empty(t, stop)
	// Worker still registered; once it becomes inactive, the next Apply
	// will emit it as stop.
	require.False(t, assignment.IsRetryDesired("1-1"))

	assignment.SetActive("1-1", false)
	_, stop = assignment.ApplyAll(nil)
	require.Equal(t, []string{"1-1"}, stop)
}

func TestClientAssignmentApplyMatchedAssignsFixedBusID(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment([]option.USBIPDeviceMatch{{BusID: "1-1"}})
	entries := []DeviceEntry{standardTestDeviceEntry("1-1")}

	next, prev := assignment.ApplyMatched(entries, nil)
	require.Equal(t, []string{"1-1"}, next)
	require.Equal(t, []string{""}, prev)
}

func TestClientAssignmentApplyMatchedReassignsHigherPriorityWhenDeviceReturns(t *testing.T) {
	t.Parallel()

	matches := []option.USBIPDeviceMatch{
		{VendorID: 0x1d6b, ProductID: 0x0002},
		{VendorID: 0x1d6b, ProductID: 0x0002},
	}
	assignment := newClientAssignment(matches)
	entries := []DeviceEntry{
		standardTestDeviceEntry("1-1"),
		standardTestDeviceEntry("2-1"),
	}

	next, _ := assignment.ApplyMatched(entries, nil)
	sort.Strings(next)
	require.Equal(t, []string{"1-1", "2-1"}, next)

	// Drop the first device. Its slot is vacated; the second worker
	// keeps its busid since target 1 still matches 2-1.
	missing := []DeviceEntry{standardTestDeviceEntry("2-1")}
	next, prev := assignment.ApplyMatched(missing, nil)
	require.Equal(t, []string{"", "2-1"}, next)
	sort.Strings(prev)
	require.Equal(t, []string{"1-1", "2-1"}, prev)

	// Device returns and fills the empty slot.
	next, prev = assignment.ApplyMatched(entries, nil)
	require.Equal(t, []string{"1-1", "2-1"}, next)
	require.Equal(t, []string{"", "2-1"}, prev)
}

func TestClientAssignmentDropRegisteredRemovesBusID(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment(nil)
	_, _ = assignment.ApplyAll([]DeviceEntry{standardTestDeviceEntry("1-1")})
	require.True(t, assignment.IsRetryDesired("1-1"))

	assignment.DropRegistered("1-1")
	require.False(t, assignment.IsRetryDesired("1-1"))
}

func TestClientAssignmentClearRegisteredClearsRegistry(t *testing.T) {
	t.Parallel()

	assignment := newClientAssignment(nil)
	_, _ = assignment.ApplyAll([]DeviceEntry{
		standardTestDeviceEntry("1-1"),
		standardTestDeviceEntry("2-1"),
	})
	require.True(t, assignment.IsRetryDesired("1-1"))
	require.True(t, assignment.IsRetryDesired("2-1"))

	assignment.ClearRegistered()
	require.False(t, assignment.IsRetryDesired("1-1"))
	require.False(t, assignment.IsRetryDesired("2-1"))
}
