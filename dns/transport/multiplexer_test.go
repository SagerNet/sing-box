package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	mDNS "github.com/miekg/dns"
)

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
