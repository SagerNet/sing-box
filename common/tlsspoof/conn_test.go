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

// realClientHello is a captured Chrome ClientHello for github.com. Tests that
// stack tlsspoof.Conn on top of tf.Conn still need a parseable payload to
// exercise the fragment transform.
const realClientHello = "16030105f8010005f403036e35de7389a679c54029cf452611f2211c70d9ac3897271de589ab6155f8e4ab20637d225f1ef969ad87ed78bfb9d171300bcb1703b6f314ccefb964f79b7d0961002a0a0a130213031301c02cc02bcca9c030c02fcca8c00ac009c014c013009d009c0035002fc008c012000a01000581baba00000000000f000d00000a6769746875622e636f6d00170000ff01000100000a000e000c3a3a11ec001d001700180019000b000201000010000e000c02683208687474702f312e31000500050100000000000d00160014040308040401050308050805050108060601020100120000003304ef04ed3a3a00010011ec04c0aeb2250c092a3463161cccb29d9183331a424964248579507ed23a180b0ceab2a5f5d9ce41547e497a89055471ea572867ba3a1fc3c9e45025274a20f60c6b60e62476b6afed0403af59ab83660ef4112ae20386a602010d0a5d454c0ed34c84ed4423e750213e6a2baab1bf9c4367a6007ab40a33d95220c2dcaa44f257024a5626b545db0510f4311b1a60714154909c6a61fdfca011fb2626d657aeb6070bf078508babe3b584555013e34acc56198ed4663742b3155a664a9901794c4586820a7dc162c01827291f3792e1237f801a8d1ef096013c181c4a58d2f6859ba75022d18cc4418bd4f351d5c18f83a58857d05af860c4b9ac018a5b63f17184e591532c6bc2cf2215d4a282c8a8a4f6f7aee110422c8bc9ebd3b1d609c568523aaae555db320e6c269473d87af38c256cbb9febc20aea6380c32a8916f7a373c8b1e37554e3260bf6621f6b804ee80b3c516b1d01985bf4c603b6daa9a5991de6a7a29f3a7122b8afb843a7660110fce62b43c615f5bcc2db688ba012649c0952b0a2c031e732d2b454c6b2968683cb8d244be2c9a7fa163222979eaf92722b92b862d81a3d94450c2b60c318421ebb4307c42d1f0473592a5c30e42039cc68cda9721e61aa63f49def17c15221680ed444896340133bbee67556f56b9f9d78a4df715f926a12add0cc9c862e46ea8b7316ae468282c18601b2771c9c9322f982228cf93effaacd3f80cbd12bce5fc36f56e2a3caf91e578a5fae00c9b23a8ed1a66764f4433c3628a70b8f0a6196adc60a4cb4226f07ba4c6b363fe9065563bfc1347452946386bab488686e837ab979c64f9047417fca635fe1bb4f074f256cc8af837c7b455e280426547755af90a61640169ef180aea3a77e662bb6dac1b6c3696027129b1a5edf495314e9c7f4b6110e16378ec893fa24642330a40aba1a85326101acb97c620fd8d71389e69eaed7bdb01bbe1fd428d66191150c7b2cd1ad4257391676a82ba8ce07fb2667c3b289f159003a7c7bc31d361b7b7f49a802961739d950dfcc0fa1c7abce5abdd2245101da391151490862028110465950b9e9c03d08a90998ab83267838d2e74a0593bc81f74cdf734519a05b351c0e5488c68dd810e6e9142ccc1e2f4a7f464297eb340e27acc6b9d64e12e38cce8492b3d939140b5a9e149a75597f10a23874c84323a07cdd657274378f887c85c4259b9c04cd33ba58ed630ef2a744f8e19dd34843dff331d2a6be7e2332c599289cd248a611c73d7481cd4a9bd43449a3836f14b2af18a1739e17999e4c67e85cc5bcecabb14185e5bcaff3c96098f03dc5aba819f29587758f49f940585354a2a780830528d68ccd166920dadcaa25cab5fc1907272a826aba3f08bc6b88757776812ecb6c7cec69a223ec0a13a7b62a2349a0f63ed7a27a3b15ba21d71fe6864ec6e089ae17cadd433fa3138f7ee24353c11365818f8fc34f43a05542d18efaac24bfccc1f748a0cc1a67ad379468b76fd34973dba785f5c91d618333cd810fe0700d1bbc8422029782628070a624c52c5309a4a64d625b11f8033ab28df34a1add297517fcc06b92b6817b3c5144438cf260867c57bde68c8c4b82e6a135ef676a52fbae5708002a404e6189a60e2836de565ad1b29e3819e5ed49f6810bcb28e1bd6de57306f94b79d9dae1cc4624d2a068499beef81cd5fe4b76dcbfff2a2008001d002001976128c6d5a934533f28b9914d2480aab2a8c1ab03d212529ce8b27640a716002d00020101002b000706caca03040303001b00030200015a5a000100"

func decodeClientHello(t *testing.T) []byte {
	t.Helper()
	payload, err := hex.DecodeString(realClientHello)
	require.NoError(t, err)
	return payload
}

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
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	spoofer := &fakeSpoofer{}
	wrapped, err := newConn(client, spoofer, "letsencrypt.org")
	require.NoError(t, err)

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
	payload := decodeClientHello(t)

	client, server := net.Pipe()
	spoofer := &fakeSpoofer{}
	wrapped, err := newConn(client, spoofer, "letsencrypt.org")
	require.NoError(t, err)

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

// TestConn_Write_SurfacesCloseError guards against the defer pattern silently
// dropping the spoofer's Close() error on the success path.
func TestConn_Write_SurfacesCloseError(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	spoofer := &fakeSpoofer{closeErr: errSpoofClose}
	wrapped, err := newConn(client, spoofer, "letsencrypt.org")
	require.NoError(t, err)

	go func() { _, _ = io.ReadAll(server) }()

	_, err = wrapped.Write([]byte("trigger inject"))
	require.ErrorIs(t, err, errSpoofClose,
		"Close() error must be wrapped into Write's return")
}

func TestConn_NewConn_RejectsEmptySNI(t *testing.T) {
	t.Parallel()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	_, err := newConn(client, &fakeSpoofer{}, "")
	require.Error(t, err, "empty SNI must fail at construction")
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
	wrapped, err := newConn(fragConn, spoofer, "letsencrypt.org")
	require.NoError(t, err)

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err = wrapped.Write(payload)
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
	wrapped, err := newConn(fragConn, spoofer, "letsencrypt.org")
	require.NoError(t, err)

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err = wrapped.Write(payload)
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
	wrapped, err := newConn(fragConn, spoofer, "letsencrypt.org")
	require.NoError(t, err)

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err = wrapped.Write(payload)
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
	wrapped, err := newConn(fragConn, spoofer, "letsencrypt.org")
	require.NoError(t, err)

	serverRead := make(chan []byte, 1)
	go func() { serverRead <- readAll(t, server) }()

	_, err = wrapped.Write(payload)
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
