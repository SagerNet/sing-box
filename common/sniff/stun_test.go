package sniff_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

func TestSniffSTUN(t *testing.T) {
	t.Parallel()
	packet, err := hex.DecodeString("000100002112a44224b1a025d0c180c484341306")
	require.NoError(t, err)
	metadata, err := sniff.STUNMessage(context.Background(), packet)
	require.NoError(t, err)
	require.Equal(t, metadata.Protocol, C.ProtocolSTUN)
}

func FuzzSniffSTUN(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		if _, err := sniff.STUNMessage(context.Background(), data); err == nil {
			t.Fail()
		}
	})
}
