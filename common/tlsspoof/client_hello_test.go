package tlsspoof

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	tf "github.com/sagernet/sing-box/common/tlsfragment"

	"github.com/stretchr/testify/require"
)

// x25519MLKEM768 is the IANA code point for the post-quantum hybrid named
// group (0x11EC). The fake ClientHello must never carry it — its 1184-byte
// key share is the reason kernel-generated ClientHellos exceed one MSS, and
// the reason this builder has to force CurvePreferences.
const x25519MLKEM768 uint16 = 0x11EC

func TestBuildFakeClientHello_ParsesWithSNI(t *testing.T) {
	t.Parallel()
	record, err := buildFakeClientHello("example.com")
	require.NoError(t, err)

	serverName := tf.IndexTLSServerName(record)
	require.NotNil(t, serverName, "output must parse as a ClientHello")
	require.Equal(t, "example.com", serverName.ServerName)

	recordLen := binary.BigEndian.Uint16(record[3:5])
	require.Equal(t, len(record)-5, int(recordLen),
		"record length header must match on-wire record size")
	handshakeLen := int(record[6])<<16 | int(record[7])<<8 | int(record[8])
	require.Equal(t, len(record)-5-4, handshakeLen,
		"handshake length header must match handshake body size")
}

// TestBuildFakeClientHello_FitsOneSegment is the regression guard for the
// whole point of the rewrite: the fake must never need fragmenting on a
// standard 1500-byte path MTU. 1200 leaves ~260 bytes for IP+TCP headers and
// a generous safety margin — the X25519MLKEM768 ClientHello this replaces
// hit ~1400+.
func TestBuildFakeClientHello_FitsOneSegment(t *testing.T) {
	t.Parallel()
	for _, sni := range []string{"a.io", "example.com", strings.Repeat("a", 253)} {
		record, err := buildFakeClientHello(sni)
		require.NoError(t, err, "sni=%q", sni)
		require.Less(t, len(record), 1200, "sni=%q built %d bytes", sni, len(record))
	}
}

// TestBuildFakeClientHello_NoPostQuantumKeyShare catches regressions that
// would accidentally pull an X25519MLKEM768 key share (the reason the prior
// implementation had to fragment) back into the fake — e.g. if CurvePreferences
// stopped being respected by a future Go version.
func TestBuildFakeClientHello_NoPostQuantumKeyShare(t *testing.T) {
	t.Parallel()
	record, err := buildFakeClientHello("example.com")
	require.NoError(t, err)

	var needle [2]byte
	binary.BigEndian.PutUint16(needle[:], x25519MLKEM768)
	require.False(t, bytes.Contains(record, needle[:]),
		"output must not contain the X25519MLKEM768 code point (0x%04x)", x25519MLKEM768)
}

// TestBuildFakeClientHello_RandomizesPerCall ensures crypto/tls generates a
// fresh random + session_id + key_share on every call, as required to avoid
// trivial fingerprinting of the spoof.
func TestBuildFakeClientHello_RandomizesPerCall(t *testing.T) {
	t.Parallel()
	first, err := buildFakeClientHello("example.com")
	require.NoError(t, err)
	second, err := buildFakeClientHello("example.com")
	require.NoError(t, err)
	require.NotEqual(t, first, second,
		"repeated calls must produce distinct bytes (random/session_id/key_share must vary)")
}

func TestBuildFakeClientHello_RejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := buildFakeClientHello("")
	require.Error(t, err)
}
