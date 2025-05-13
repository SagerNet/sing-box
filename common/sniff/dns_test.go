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

func TestSniffDNS(t *testing.T) {
	t.Parallel()
	query, err := hex.DecodeString("740701000001000000000000012a06676f6f676c6503636f6d0000010001")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.DomainNameQuery(context.TODO(), &metadata, query)
	require.NoError(t, err)
	require.Equal(t, C.ProtocolDNS, metadata.Protocol)
}

func TestSniffStreamDNS(t *testing.T) {
	t.Parallel()
	query, err := hex.DecodeString("001e740701000001000000000000012a06676f6f676c6503636f6d0000010001")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.StreamDomainNameQuery(context.TODO(), &metadata, bytes.NewReader(query))
	require.NoError(t, err)
	require.Equal(t, C.ProtocolDNS, metadata.Protocol)
}

func TestSniffIncompleteStreamDNS(t *testing.T) {
	t.Parallel()
	query, err := hex.DecodeString("001e740701000001000000000000")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.StreamDomainNameQuery(context.TODO(), &metadata, bytes.NewReader(query))
	require.ErrorIs(t, err, sniff.ErrNeedMoreData)
}

func TestSniffNotStreamDNS(t *testing.T) {
	t.Parallel()
	query, err := hex.DecodeString("001e740701000000000000000000")
	require.NoError(t, err)
	var metadata adapter.InboundContext
	err = sniff.StreamDomainNameQuery(context.TODO(), &metadata, bytes.NewReader(query))
	require.NotEmpty(t, err)
	require.NotErrorIs(t, err, sniff.ErrNeedMoreData)
}
