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

func TestSniffDTLSClientHello(t *testing.T) {
	t.Parallel()
	packet, err := hex.DecodeString("16fefd0000000000000000007e010000720000000000000072fefd668a43523798e064bd806d0c87660de9c611a59bbdfc3892c4e072d94f2cafc40000000cc02bc02fc00ac014c02cc0300100003c000d0010000e0403050306030401050106010807ff01000100000a00080006001d00170018000b00020100000e000900060008000700010000170000")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.DTLSRecord(context.Background(), &metadata, packet)
	require.NoError(t, err)
	require.Equal(t, metadata.Protocol, C.ProtocolDTLS)
}

func TestSniffDTLSClientApplicationData(t *testing.T) {
	t.Parallel()
	packet, err := hex.DecodeString("17fefd000100000000000100440001000000000001a4f682b77ecadd10f3f3a2f78d90566212366ff8209fd77314f5a49352f9bb9bd12f4daba0b4736ae29e46b9714d3b424b3e6d0234736619b5aa0d3f")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.DTLSRecord(context.Background(), &metadata, packet)
	require.NoError(t, err)
	require.Equal(t, metadata.Protocol, C.ProtocolDTLS)
}
