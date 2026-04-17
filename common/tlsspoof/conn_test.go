package tlsspoof

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"testing"
	"time"

	tf "github.com/sagernet/sing-box/common/tlsfragment"

	"github.com/stretchr/testify/require"
)

type fakeSpoofer struct {
	injected [][]byte
	err      error
	closeErr error
}

func (f *fakeSpoofer) Inject(payload []byte) error {
	if f.err != nil {
		return f.err
	}
	f.injected = append(f.injected, append([]byte(nil), payload...))
	return nil
}

func (f *fakeSpoofer) Close() error {
	return f.closeErr
}

func readAll(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	data, err := io.ReadAll(conn)
	require.NoError(t, err)
	return data
}

func TestConn_Write_InjectsThenForwards(t *testing.T) {
	t.Parallel()
	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	client, server := net.Pipe()
	spoofer := &fakeSpoofer{}
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() {
		serverRead <- readAll(t, server)
	}()

	n, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	require.NoError(t, wrapped.Close())

	forwarded := <-serverRead
	require.Equal(t, payload, forwarded, "underlying conn must receive the real ClientHello unchanged")
	require.Len(t, spoofer.injected, 1)

	injected := spoofer.injected[0]
	serverName := tf.IndexTLSServerName(injected)
	require.NotNil(t, serverName, "injected payload must parse as ClientHello")
	require.Equal(t, "letsencrypt.org", serverName.ServerName)
}

func TestConn_Write_SecondWriteDoesNotInject(t *testing.T) {
	t.Parallel()
	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	client, server := net.Pipe()
	spoofer := &fakeSpoofer{}
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() {
		serverRead <- readAll(t, server)
	}()

	_, err = wrapped.Write(payload)
	require.NoError(t, err)
	_, err = wrapped.Write([]byte("second"))
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())

	forwarded := <-serverRead
	require.Equal(t, append(append([]byte(nil), payload...), []byte("second")...), forwarded)
	require.Len(t, spoofer.injected, 1)
}

func TestConn_Write_NonClientHelloReturnsError(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	spoofer := &fakeSpoofer{}
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	_, err := wrapped.Write([]byte("not a ClientHello"))
	require.Error(t, err)
	require.Empty(t, spoofer.injected)
}

// TestConn_Write_SurfacesCloseError guards against the defer pattern silently
// dropping the spoofer's Close() error on the success path.
func TestConn_Write_SurfacesCloseError(t *testing.T) {
	t.Parallel()
	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	spoofer := &fakeSpoofer{closeErr: errSpoofClose}
	wrapped := NewConn(client, spoofer, "letsencrypt.org")

	go func() { _, _ = io.ReadAll(server) }()

	_, err = wrapped.Write(payload)
	require.ErrorIs(t, err, errSpoofClose,
		"Close() error must be wrapped into Write's return")
}

var errSpoofClose = errTest("spoof-close-failed")

type errTest string

func (e errTest) Error() string { return string(e) }

// recordingConn intercepts each Write call so tests can assert how many
// downstream writes occurred and in what order with respect to spoof
// injection. It does not implement WithUpstream, so tf.Conn's
// N.UnwrapReader(conn).(*net.TCPConn) returns nil and fragment-mode falls
// back to its plain Write + time.Sleep path — which is what we want to
// exercise over a net.Pipe.
type recordingConn struct {
	net.Conn
	writes   [][]byte
	timeline *[]string
}

func (c *recordingConn) Write(p []byte) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), p...))
	if c.timeline != nil {
		*c.timeline = append(*c.timeline, "write")
	}
	return c.Conn.Write(p)
}

type tlsRecord struct {
	contentType byte
	payload     []byte
}

func parseTLSRecords(t *testing.T, data []byte) []tlsRecord {
	t.Helper()
	var records []tlsRecord
	for len(data) > 0 {
		require.GreaterOrEqual(t, len(data), 5, "record header incomplete")
		recordLen := int(binary.BigEndian.Uint16(data[3:5]))
		require.GreaterOrEqual(t, len(data), 5+recordLen, "record payload truncated")
		records = append(records, tlsRecord{
			contentType: data[0],
			payload:     append([]byte(nil), data[5:5+recordLen]...),
		})
		data = data[5+recordLen:]
	}
	return records
}

// TestConn_StackedWithRecordFragment mirrors the wrapping order that
// STDClientConfig.Client() produces when record_fragment is enabled:
// tls.Client → tlsspoof.Conn → tf.Conn → raw conn.
// Asserts the decoy is injected and the real handshake arrives split into
// multiple TLS records whose payloads reassemble to the original.
func TestConn_StackedWithRecordFragment(t *testing.T) {
	t.Parallel()
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	defer server.Close()

	fragConn := tf.NewConn(client, context.Background(), false, true, time.Millisecond)
	spoofer := &fakeSpoofer{}
	wrapped := NewConn(fragConn, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())
	forwarded := <-serverRead

	require.Len(t, spoofer.injected, 1, "spoof must inject exactly once")
	injected := tf.IndexTLSServerName(spoofer.injected[0])
	require.NotNil(t, injected, "injected payload must parse as ClientHello")
	require.Equal(t, "letsencrypt.org", injected.ServerName)

	records := parseTLSRecords(t, forwarded)
	require.Greater(t, len(records), 1, "record_fragment must produce multiple records")
	var reassembled []byte
	for _, r := range records {
		require.Equal(t, byte(0x16), r.contentType, "all records must be handshake")
		reassembled = append(reassembled, r.payload...)
	}
	require.Equal(t, payload[5:], reassembled, "record payloads must reassemble to original handshake")
}

// TestConn_StackedWithPacketFragment is the primary regression test for the
// fragment-only gate fix in STDClientConfig.Client(). It verifies that
// packet-level fragmentation combined with spoof produces:
//   - one spoof injection carrying the decoy SNI,
//   - multiple separate writes to the underlying conn,
//   - an unmodified byte stream when those writes are concatenated
//     (no extra record framing).
func TestConn_StackedWithPacketFragment(t *testing.T) {
	t.Parallel()
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	defer server.Close()

	rc := &recordingConn{Conn: client}
	fragConn := tf.NewConn(rc, context.Background(), true, false, time.Millisecond)
	spoofer := &fakeSpoofer{}
	wrapped := NewConn(fragConn, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())
	forwarded := <-serverRead

	require.Len(t, spoofer.injected, 1, "spoof must inject exactly once")
	injected := tf.IndexTLSServerName(spoofer.injected[0])
	require.NotNil(t, injected)
	require.Equal(t, "letsencrypt.org", injected.ServerName)

	require.Greater(t, len(rc.writes), 1, "fragment must split the ClientHello into multiple writes")
	require.Equal(t, payload, bytes.Join(rc.writes, nil),
		"concatenated writes must equal original bytes (no extra framing)")
	require.Equal(t, payload, forwarded)
}

// TestConn_StackedWithBothFragment exercises the combination that produces
// the strongest obfuscation: each chunk becomes its own TLS record and its
// own TCP write.
func TestConn_StackedWithBothFragment(t *testing.T) {
	t.Parallel()
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	defer server.Close()

	rc := &recordingConn{Conn: client}
	fragConn := tf.NewConn(rc, context.Background(), true, true, time.Millisecond)
	spoofer := &fakeSpoofer{}
	wrapped := NewConn(fragConn, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())
	forwarded := <-serverRead

	require.Len(t, spoofer.injected, 1)
	injected := tf.IndexTLSServerName(spoofer.injected[0])
	require.NotNil(t, injected)
	require.Equal(t, "letsencrypt.org", injected.ServerName)

	require.Greater(t, len(rc.writes), 1, "split-packet must produce multiple writes")
	records := parseTLSRecords(t, forwarded)
	require.Greater(t, len(records), 1, "split-record must produce multiple records")
	var reassembled []byte
	for _, r := range records {
		require.Equal(t, byte(0x16), r.contentType)
		reassembled = append(reassembled, r.payload...)
	}
	require.Equal(t, payload[5:], reassembled,
		"record payloads must reassemble to the original handshake")
}

// trackingSpoofer adds the spoof injection to a shared event timeline so
// TestConn_StackedInjectionOrder can prove the decoy precedes the first
// downstream write.
type trackingSpoofer struct {
	injected [][]byte
	timeline *[]string
}

func (s *trackingSpoofer) Inject(payload []byte) error {
	s.injected = append(s.injected, append([]byte(nil), payload...))
	*s.timeline = append(*s.timeline, "inject")
	return nil
}

func (s *trackingSpoofer) Close() error { return nil }

// TestConn_StackedInjectionOrder asserts the documented wire order: the
// decoy injection happens before any write reaches the underlying conn.
func TestConn_StackedInjectionOrder(t *testing.T) {
	t.Parallel()
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	defer server.Close()

	var timeline []string
	rc := &recordingConn{Conn: client, timeline: &timeline}
	fragConn := tf.NewConn(rc, context.Background(), true, true, time.Millisecond)
	spoofer := &trackingSpoofer{timeline: &timeline}
	wrapped := NewConn(fragConn, spoofer, "letsencrypt.org")

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err := wrapped.Write(payload)
	require.NoError(t, err)
	require.NoError(t, wrapped.Close())
	<-serverRead

	require.NotEmpty(t, timeline)
	require.Equal(t, "inject", timeline[0], "decoy must be injected before any downstream write")
	require.Contains(t, timeline[1:], "write", "at least one downstream write must follow the inject")
}

func TestParseMethod(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want Method
		ok   bool
	}{
		"":               {MethodWrongSequence, true},
		"wrong-sequence": {MethodWrongSequence, true},
		"wrong-checksum": {MethodWrongChecksum, true},
		"nonsense":       {0, false},
	}
	for input, expected := range cases {
		m, err := ParseMethod(input)
		if !expected.ok {
			require.Error(t, err, "input=%q", input)
			continue
		}
		require.NoError(t, err, "input=%q", input)
		require.Equal(t, expected.want, m, "input=%q", input)
	}
}
