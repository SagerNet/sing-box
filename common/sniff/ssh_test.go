package sniff_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

func TestSniffSSH(t *testing.T) {
	t.Parallel()

	pkt, err := hex.DecodeString("5353482d322e302d64726f70626561720d0a000001a40a1492892570d1223aef61b0d647972c8bd30000009f637572766532353531392d7368613235362c637572766532353531392d736861323536406c69627373682e6f72672c6469666669652d68656c6c6d616e2d67726f757031342d7368613235362c6469666669652d68656c6c6d616e2d67726f757031342d736861312c6b6578677565737332406d6174742e7563632e61736e2e61752c6b65782d7374726963742d732d763030406f70656e7373682e636f6d000000207373682d656432353531392c7273612d736861322d3235362c7373682d7273610000003363686163686132302d706f6c7931333035406f70656e7373682e636f6d2c6165733132382d6374722c6165733235362d6374720000003363686163686132302d706f6c7931333035406f70656e7373682e636f6d2c6165733132382d6374722c6165733235362d63747200000017686d61632d736861312c686d61632d736861322d32353600000017686d61632d736861312c686d61632d736861322d323536000000046e6f6e65000000046e6f6e65000000000000000000000000002aa6ed090585b7d635b6")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.SSH(context.TODO(), &metadata, bytes.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, C.ProtocolSSH, metadata.Protocol)
	require.Equal(t, "dropbear", metadata.Client)
}

func TestSniffIncompleteSSH(t *testing.T) {
	t.Parallel()

	pkt, err := hex.DecodeString("5353482d322e30")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.SSH(context.TODO(), &metadata, bytes.NewReader(pkt))
	require.ErrorIs(t, err, sniff.ErrNeedMoreData)
}

func TestSniffNotSSH(t *testing.T) {
	t.Parallel()

	pkt, err := hex.DecodeString("5353482d322e31")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.SSH(context.TODO(), &metadata, bytes.NewReader(pkt))
	require.NotEmpty(t, err)
	require.NotErrorIs(t, err, sniff.ErrNeedMoreData)
}
