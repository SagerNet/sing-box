package tlsspoof

import (
	"encoding/hex"
	"io"
	"net"
	"testing"

	tf "github.com/sagernet/sing-box/common/tlsfragment"

	"github.com/stretchr/testify/require"
)

type fakeSpoofer struct {
	injected [][]byte
	err      error
}

func (f *fakeSpoofer) Inject(payload []byte) error {
	if f.err != nil {
		return f.err
	}
	f.injected = append(f.injected, append([]byte(nil), payload...))
	return nil
}

func (f *fakeSpoofer) Close() error {
	return nil
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
