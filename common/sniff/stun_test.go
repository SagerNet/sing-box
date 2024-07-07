package sniff_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

func TestSniffSTUN(t *testing.T) {
	t.Parallel()
	packet, err := hex.DecodeString("000100002112a44224b1a025d0c180c484341306")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.STUNMessage(context.Background(), &metadata, packet)
	require.NoError(t, err)
	require.Equal(t, metadata.Protocol, C.ProtocolSTUN)
}

func FuzzSniffSTUN(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		var metadata adapter.InboundContext
		if err := sniff.STUNMessage(context.Background(), &metadata, data); err == nil {
			t.Fail()
		}
	})
}
