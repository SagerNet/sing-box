package tls

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	tf "github.com/sagernet/sing-box/common/tlsfragment"
	"github.com/sagernet/sing-box/common/tlsspoof"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestParseTLSSpoofOptions_Disabled(t *testing.T) {
	t.Parallel()
	spoof, method, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{})
	require.NoError(t, err)
	require.Empty(t, spoof)
	require.Equal(t, tlsspoof.MethodWrongSequence, method)
}

func TestParseTLSSpoofOptions_MethodWithoutSpoof(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		SpoofMethod: tlsspoof.MethodNameWrongChecksum,
	})
	require.Error(t, err)
}

func TestParseTLSSpoofOptions_IPLiteralRejected(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("1.2.3.4", option.OutboundTLSOptions{
		Spoof: "example.com",
	})
	require.Error(t, err)
}

func TestParseTLSSpoofOptions_EmptyServerNameRejected(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("", option.OutboundTLSOptions{
		Spoof: "example.com",
	})
	require.Error(t, err)
}

func TestParseTLSSpoofOptions_DisableSNIRejected(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		Spoof:      "decoy.com",
		DisableSNI: true,
	})
	require.Error(t, err)
}

// TestParseTLSSpoofOptions_RejectsSameSNI is the primary regression test for
// the "spoofed packet contains the original SNI" bug report: when a user
// configures spoof equal to server_name, the rewriter produces a byte-identical
// record, so the fake and real ClientHellos on the wire look the same. Reject
// at parse time.
func TestParseTLSSpoofOptions_RejectsSameSNI(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		Spoof: "example.com",
	})
	require.Error(t, err)

	_, _, err = parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		Spoof: "EXAMPLE.com",
	})
	require.Error(t, err, "comparison must be case-insensitive")
}

func TestParseTLSSpoofOptions_UnknownMethodRejected(t *testing.T) {
	t.Parallel()
	_, _, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		Spoof:       "decoy.com",
		SpoofMethod: "nonsense",
	})
	require.Error(t, err)
}

func TestParseTLSSpoofOptions_DistinctSNIAccepted(t *testing.T) {
	t.Parallel()
	if !tlsspoof.PlatformSupported {
		t.Skip("tlsspoof not supported on this platform")
	}
	spoof, method, err := parseTLSSpoofOptions("example.com", option.OutboundTLSOptions{
		Spoof:       "decoy.com",
		SpoofMethod: tlsspoof.MethodNameWrongSequence,
	})
	require.NoError(t, err)
	require.Equal(t, "decoy.com", spoof)
	require.Equal(t, tlsspoof.MethodWrongSequence, method)
}

// The following tests guard the wrap gate in STDClientConfig.Client():
// tf.Conn must wrap the underlying connection whenever either `fragment` or
// `record_fragment` is set, so that TLS fragmentation coexists with features
// like tls_spoof that layer on top of tf.Conn.

func newSTDClientConfigForGateTest(fragment, recordFragment bool) *STDClientConfig {
	return &STDClientConfig{
		ctx:            context.Background(),
		config:         &tls.Config{ServerName: "example.com", InsecureSkipVerify: true},
		fragment:       fragment,
		recordFragment: recordFragment,
	}
}

func TestSTDClient_Client_NoFragment_DoesNotWrap(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	wrapped, err := newSTDClientConfigForGateTest(false, false).Client(client)
	require.NoError(t, err)
	_, isTF := wrapped.NetConn().(*tf.Conn)
	require.False(t, isTF, "no fragment flags: must not wrap with tf.Conn")
}

func TestSTDClient_Client_FragmentOnly_Wraps(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	wrapped, err := newSTDClientConfigForGateTest(true, false).Client(client)
	require.NoError(t, err)
	_, isTF := wrapped.NetConn().(*tf.Conn)
	require.True(t, isTF, "fragment=true: must wrap with tf.Conn")
}

func TestSTDClient_Client_RecordFragmentOnly_Wraps(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	wrapped, err := newSTDClientConfigForGateTest(false, true).Client(client)
	require.NoError(t, err)
	_, isTF := wrapped.NetConn().(*tf.Conn)
	require.True(t, isTF, "record_fragment=true: must wrap with tf.Conn")
}

func TestSTDClient_Client_BothFragment_Wraps(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	wrapped, err := newSTDClientConfigForGateTest(true, true).Client(client)
	require.NoError(t, err)
	_, isTF := wrapped.NetConn().(*tf.Conn)
	require.True(t, isTF, "both fragment flags: must wrap with tf.Conn")
}
