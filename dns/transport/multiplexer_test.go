package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	boxDNS "github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	mDNS "github.com/miekg/dns"
)

func TestTCPTransportRetriesReadErrorOnReusedConn(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverDone := make(chan error, 1)
	go func() {
		firstConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		firstRequest, readErr := ReadMessage(firstConn)
		if readErr != nil {
			firstConn.Close()
			serverDone <- readErr
			return
		}
		firstResponse := new(mDNS.Msg)
		firstResponse.SetReply(firstRequest)
		writeErr := WriteMessage(firstConn, firstRequest.Id, firstResponse)
		if writeErr != nil {
			firstConn.Close()
			serverDone <- writeErr
			return
		}
		_, readErr = ReadMessage(firstConn)
		firstConn.Close()
		if readErr != nil {
			serverDone <- readErr
			return
		}
		secondConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer secondConn.Close()
		secondRequest, readErr := ReadMessage(secondConn)
		if readErr != nil {
			serverDone <- readErr
			return
		}
		secondResponse := new(mDNS.Msg)
		secondResponse.SetReply(secondRequest)
		serverDone <- WriteMessage(secondConn, secondRequest.Id, secondResponse)
	}()

	transportDialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	transport := NewTCPRaw(boxDNS.NewTransportAdapter(C.DNSTypeTCP, "test", nil), transportDialer, M.SocksaddrFromNet(listener.Addr()))
	defer transport.Close()

	firstMessage := new(mDNS.Msg)
	firstMessage.SetQuestion("first.example.com.", mDNS.TypeA)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = transport.Exchange(ctx, firstMessage)
	cancel()
	if err != nil {
		t.Fatal("first query failed: ", err)
	}

	secondMessage := new(mDNS.Msg)
	secondMessage.SetQuestion("second.example.com.", mDNS.TypeAAAA)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	_, err = transport.Exchange(ctx, secondMessage)
	cancel()
	if err != nil {
		t.Fatal("second query failed: ", err)
	}
	select {
	case err = <-serverDone:
		if err != nil {
			t.Fatal("DNS server failed: ", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DNS server did not finish")
	}
}

func TestMultiplexerTimeoutInvalidatesConn(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 16)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted <- conn
		}
	}()
	multiplexer := newQueryMultiplexer(queryMultiplexerOptions{
		dial: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", listener.Addr().String())
		},
		write: func(conn net.Conn, message *mDNS.Msg, queryId uint16) error {
			return WriteMessage(conn, queryId, message)
		},
		readNext: func(conn net.Conn) (*mDNS.Msg, error) {
			return ReadMessage(conn)
		},
	})
	defer multiplexer.Close()

	message := new(mDNS.Msg)
	message.SetQuestion("example.com.", mDNS.TypeA)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, err = multiplexer.Exchange(ctx, message)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded, got ", err)
	}
	if elapsed > 2*time.Second {
		t.Fatal("timeout not enforced, took ", elapsed)
	}

	firstConn := <-accepted
	firstConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.Copy(io.Discard, firstConn)
	if err != nil {
		t.Fatal("expected the client side to close the connection, got ", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	multiplexer.Exchange(ctx2, message)
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("expected a fresh connection for the second query")
	}
}

func TestMultiplexerSlowQueryKeepsActiveConn(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 16)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted <- conn
			go func() {
				for {
					request, readErr := ReadMessage(conn)
					if readErr != nil {
						return
					}
					if request.Question[0].Name == "slow.example.com." {
						continue
					}
					response := new(mDNS.Msg)
					response.SetReply(request)
					WriteMessage(conn, request.Id, response)
				}
			}()
		}
	}()
	multiplexer := newQueryMultiplexer(queryMultiplexerOptions{
		dial: func(ctx context.Context) (net.Conn, error) {
			return net.Dial("tcp", listener.Addr().String())
		},
		write: func(conn net.Conn, message *mDNS.Msg, queryId uint16) error {
			return WriteMessage(conn, queryId, message)
		},
		readNext: func(conn net.Conn) (*mDNS.Msg, error) {
			return ReadMessage(conn)
		},
	})
	defer multiplexer.Close()

	slowMessage := new(mDNS.Msg)
	slowMessage.SetQuestion("slow.example.com.", mDNS.TypeA)
	slowCtx, slowCancel := context.WithTimeout(context.Background(), time.Second)
	defer slowCancel()
	slowDone := make(chan error, 1)
	go func() {
		_, slowErr := multiplexer.Exchange(slowCtx, slowMessage)
		slowDone <- slowErr
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("expected a connection for the slow query")
	}

	fastMessage := new(mDNS.Msg)
	fastMessage.SetQuestion("fast.example.com.", mDNS.TypeA)
	exchangeFast := func() {
		fastCtx, fastCancel := context.WithTimeout(context.Background(), time.Second)
		defer fastCancel()
		_, fastErr := multiplexer.Exchange(fastCtx, fastMessage)
		if fastErr != nil {
			t.Fatal("fast query failed: ", fastErr)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if !time.Now().Before(deadline) {
			t.Fatal("slow query did not complete")
		}
		exchangeFast()
		select {
		case slowErr := <-slowDone:
			if !errors.Is(slowErr, context.DeadlineExceeded) {
				t.Fatal("expected deadline exceeded for slow query, got ", slowErr)
			}
			exchangeFast()
			if len(accepted) > 0 {
				t.Fatal("slow query timeout must not replace the active connection")
			}
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}
