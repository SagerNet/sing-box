package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
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
		retryReadError: true,
	})
	defer multiplexer.Close()

	firstMessage := new(mDNS.Msg)
	firstMessage.SetQuestion("first.example.com.", mDNS.TypeA)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = multiplexer.Exchange(ctx, firstMessage)
	cancel()
	if err != nil {
		t.Fatal("first query failed: ", err)
	}

	secondMessage := new(mDNS.Msg)
	secondMessage.SetQuestion("second.example.com.", mDNS.TypeAAAA)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	_, err = multiplexer.Exchange(ctx, secondMessage)
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

func newTestTCPTransport(t *testing.T, listener net.Listener) *TCPTransport {
	transportDialer, err := dialer.NewDefault(context.Background(), option.DialerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return NewTCPRaw(boxDNS.NewTransportAdapter(C.DNSTypeTCP, "test", nil), transportDialer, M.SocksaddrFromNet(listener.Addr()))
}

func testExchange(transport *TCPTransport, questionName string) error {
	message := new(mDNS.Msg)
	message.SetQuestion(questionName, mDNS.TypeA)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := transport.Exchange(ctx, message)
	return err
}

func TestTCPTransportSingleQueryServer(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var accepted atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer conn.Close()
				request, readErr := ReadMessage(conn)
				if readErr != nil {
					return
				}
				response := new(mDNS.Msg)
				response.SetReply(request)
				WriteMessage(conn, request.Id, response)
			}()
		}
	}()

	transport := newTestTCPTransport(t, listener)
	defer transport.Close()

	const queryCount = 8
	results := make(chan error, queryCount)
	for range queryCount {
		go func() {
			results <- testExchange(transport, "example.com.")
		}()
	}
	for range queryCount {
		err = <-results
		if err != nil {
			t.Fatal("query failed: ", err)
		}
	}
	deadline := time.Now().Add(time.Second)
	for accepted.Load() < queryCount+1 {
		if time.Now().After(deadline) {
			t.Fatal("expected a probe connection, accepted ", accepted.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if count := accepted.Load(); count != queryCount+1 {
		t.Fatal("expected one connection per query plus probe, accepted ", count)
	}
}

func TestTCPTransportProbeEnablesReuse(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var maxServedOnConn atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				var served int32
				for {
					request, readErr := ReadMessage(conn)
					if readErr != nil {
						return
					}
					served++
					for {
						current := maxServedOnConn.Load()
						if served <= current || maxServedOnConn.CompareAndSwap(current, served) {
							break
						}
					}
					response := new(mDNS.Msg)
					response.SetReply(request)
					WriteMessage(conn, request.Id, response)
				}
			}()
		}
	}()

	transport := newTestTCPTransport(t, listener)
	defer transport.Close()

	deadline := time.Now().Add(3 * time.Second)
	for maxServedOnConn.Load() < 3 {
		if time.Now().After(deadline) {
			t.Fatal("reuse was not enabled after successful probe")
		}
		err = testExchange(transport, "example.com.")
		if err != nil {
			t.Fatal("query failed: ", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	const burstCount = 5
	results := make(chan error, burstCount)
	for range burstCount {
		go func() {
			results <- testExchange(transport, "example.com.")
		}()
	}
	for range burstCount {
		err = <-results
		if err != nil {
			t.Fatal("burst query failed: ", err)
		}
	}
}

func TestTCPTransportDemotesBrokenReuse(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var accepted atomic.Int32
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer conn.Close()
				for served := 0; ; served++ {
					request, readErr := ReadMessage(conn)
					if readErr != nil {
						return
					}
					if served >= 2 {
						return
					}
					response := new(mDNS.Msg)
					response.SetReply(request)
					WriteMessage(conn, request.Id, response)
				}
			}()
		}
	}()

	transport := newTestTCPTransport(t, listener)
	defer transport.Close()

	deadline := time.Now().Add(3 * time.Second)
	for {
		before := accepted.Load()
		err = testExchange(transport, "example.com.")
		if err != nil {
			t.Fatal("query failed: ", err)
		}
		if accepted.Load() == before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reuse was not enabled after successful probe")
		}
	}

	for range 15 {
		err = testExchange(transport, "example.com.")
		if err != nil {
			t.Fatal("query failed during demotion: ", err)
		}
	}
	if transport.multiplexer.reuseState.Load() != reuseStateUnsupported {
		t.Fatal("expected demotion to single connection mode")
	}

	time.Sleep(100 * time.Millisecond)
	before := accepted.Load()
	const singleCount = 4
	for range singleCount {
		err = testExchange(transport, "example.com.")
		if err != nil {
			t.Fatal("query failed after demotion: ", err)
		}
	}
	if count := accepted.Load() - before; count != singleCount {
		t.Fatal("expected one connection per query after demotion, got ", count)
	}
}

func TestTCPTransportSilentPipelineServer(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				request, readErr := ReadMessage(conn)
				if readErr != nil {
					return
				}
				conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				_, secondErr := ReadMessage(conn)
				if secondErr == nil {
					conn.SetReadDeadline(time.Time{})
					io.Copy(io.Discard, conn)
					return
				}
				var netErr net.Error
				if !errors.As(secondErr, &netErr) || !netErr.Timeout() {
					return
				}
				conn.SetReadDeadline(time.Time{})
				response := new(mDNS.Msg)
				response.SetReply(request)
				WriteMessage(conn, request.Id, response)
			}()
		}
	}()

	transport := newTestTCPTransport(t, listener)
	defer transport.Close()

	const queryCount = 5
	results := make(chan error, queryCount)
	for range queryCount {
		go func() {
			results <- testExchange(transport, "example.com.")
		}()
	}
	for range queryCount {
		err = <-results
		if err != nil {
			t.Fatal("query failed: ", err)
		}
	}

	deadline := time.Now().Add(8 * time.Second)
	for transport.multiplexer.reuseState.Load() != reuseStateUnsupported {
		if time.Now().After(deadline) {
			t.Fatal("expected probe timeout to disable reuse")
		}
		time.Sleep(100 * time.Millisecond)
	}
	err = testExchange(transport, "example.com.")
	if err != nil {
		t.Fatal("query failed after probe timeout: ", err)
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
