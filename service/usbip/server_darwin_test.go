//go:build darwin && cgo

package usbip

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDarwinServerPendingSubmitUnlinkState(t *testing.T) {
	t.Parallel()

	session := &darwinServerDataSession{
		pending: make(map[uint32]darwinServerPendingSubmit),
	}

	const endpoint uint8 = 0x81
	session.trackSubmit(7, endpoint)

	unlinkedEndpoint, active := session.markSubmitUnlinked(7)
	require.True(t, active)
	require.Equal(t, endpoint, unlinkedEndpoint)
	require.False(t, session.finishSubmit(7))

	session.trackSubmit(8, endpoint)
	require.True(t, session.finishSubmit(8))

	_, active = session.markSubmitUnlinked(8)
	require.False(t, active)
}
