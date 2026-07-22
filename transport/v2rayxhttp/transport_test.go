package v2rayxhttp

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/stretchr/testify/require"
)

type echoHandler struct{}

func (echoHandler) NewConnectionEx(_ context.Context, conn net.Conn, _ M.Socksaddr, _ M.Socksaddr, _ N.CloseHandlerFunc) {
	defer conn.Close()
	buffer := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			_, _ = conn.Write(buffer[:n])
		}
		if err != nil {
			return
		}
	}
}

var _ adapter.V2RayServerTransportHandler = echoHandler{}

func TestTransportModes(t *testing.T) {
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) { testTransportMode(t, mode) })
	}
}

func TestDownloadSettings(t *testing.T) {
	options := option.V2RayXHTTPOptions{
		Path: "/xhttp", Mode: "stream-up",
	}
	server, err := NewServer(context.Background(), logger.NOP(), options, nil, echoHandler{})
	require.NoError(t, err)
	uplinkListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	downloadListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	uplinkCounter := &countingListener{Listener: uplinkListener}
	downloadCounter := &countingListener{Listener: downloadListener}
	go server.Serve(uplinkCounter)
	go server.Serve(downloadCounter)
	defer func() {
		_ = uplinkListener.Close()
		_ = downloadListener.Close()
		_ = server.Close()
	}()

	dialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	require.NoError(t, err)
	clientOptions := options
	clientOptions.DownloadSettings = &option.V2RayXHTTPDownloadSettings{
		ServerOptions:     option.ServerOptions{Server: "127.0.0.1", ServerPort: uint16(downloadListener.Addr().(*net.TCPAddr).Port)},
		V2RayXHTTPOptions: option.V2RayXHTTPOptions{Path: "/xhttp", Mode: "stream-up"},
	}
	client, err := NewClient(context.Background(), dialer, M.SocksaddrFromNet(uplinkListener.Addr()), clientOptions, nil)
	require.NoError(t, err)
	defer client.Close()
	conn, err := client.DialContext(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	payload := []byte("xhttp download settings test")
	_, err = conn.Write(payload)
	require.NoError(t, err)
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)
	require.Equal(t, payload, response)
	require.Positive(t, uplinkCounter.accepted.Load(), "upload target was not used")
	require.Positive(t, downloadCounter.accepted.Load(), "download target was not used")
}

type countingListener struct {
	net.Listener
	accepted atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err == nil {
		l.accepted.Add(1)
	}
	return connection, err
}

func testTransportMode(t *testing.T, mode string) {
	options := option.V2RayXHTTPOptions{Path: "/xhttp", Mode: mode}
	server, err := NewServer(context.Background(), logger.NOP(), options, nil, echoHandler{})
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go server.Serve(listener)
	defer server.Close()
	dialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	require.NoError(t, err)
	client, err := NewClient(context.Background(), dialer, M.SocksaddrFromNet(listener.Addr()), options, nil)
	require.NoError(t, err)
	conn, err := client.DialContext(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	payload := []byte("xhttp transport test")
	_, err = conn.Write(payload)
	require.NoError(t, err)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)
	require.Equal(t, payload, response)
}
