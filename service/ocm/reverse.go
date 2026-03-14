package ocm

import (
	"bufio"
	"context"
	stdTLS "crypto/tls"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/hashicorp/yamux"
)

func reverseYamuxConfig() *yamux.Config {
	config := yamux.DefaultConfig()
	config.KeepAliveInterval = 15 * time.Second
	config.ConnectionWriteTimeout = 10 * time.Second
	config.MaxStreamWindowSize = 512 * 1024
	config.LogOutput = io.Discard
	return config
}

type bufferedConn struct {
	reader *bufio.Reader
	net.Conn
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

type yamuxNetListener struct {
	session *yamux.Session
}

func (l *yamuxNetListener) Accept() (net.Conn, error) {
	return l.session.Accept()
}

func (l *yamuxNetListener) Close() error {
	return l.session.Close()
}

func (l *yamuxNetListener) Addr() net.Addr {
	return l.session.Addr()
}

func (s *Service) handleReverseConnect(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "reverse-proxy" {
		writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", "missing Upgrade header")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "missing api key")
		return
	}
	clientToken := strings.TrimPrefix(authHeader, "Bearer ")
	if clientToken == authHeader {
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key format")
		return
	}

	receiverCredential := s.findReceiverCredential(clientToken)
	if receiverCredential == nil {
		s.logger.WarnContext(ctx, "reverse connect failed from ", r.RemoteAddr, ": no matching receiver credential")
		writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid reverse token")
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		s.logger.ErrorContext(ctx, "reverse connect: hijack not supported")
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "hijack not supported")
		return
	}

	conn, bufferedReadWriter, err := hijacker.Hijack()
	if err != nil {
		s.logger.ErrorContext(ctx, "reverse connect: hijack: ", err)
		return
	}

	response := "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: reverse-proxy\r\n\r\n"
	_, err = bufferedReadWriter.WriteString(response)
	if err != nil {
		conn.Close()
		s.logger.ErrorContext(ctx, "reverse connect: write upgrade response: ", err)
		return
	}
	err = bufferedReadWriter.Flush()
	if err != nil {
		conn.Close()
		s.logger.ErrorContext(ctx, "reverse connect: flush upgrade response: ", err)
		return
	}

	session, err := yamux.Client(conn, reverseYamuxConfig())
	if err != nil {
		conn.Close()
		s.logger.ErrorContext(ctx, "reverse connect: create yamux client for ", receiverCredential.tagName(), ": ", err)
		return
	}

	if !receiverCredential.setReverseSession(session) {
		session.Close()
		return
	}
	s.logger.InfoContext(ctx, "reverse connection established for ", receiverCredential.tagName(), " from ", r.RemoteAddr)

	go func() {
		<-session.CloseChan()
		receiverCredential.clearReverseSession(session)
		s.logger.WarnContext(ctx, "reverse connection lost for ", receiverCredential.tagName())
	}()
}

func (s *Service) findReceiverCredential(token string) *externalCredential {
	for _, cred := range s.allCredentials {
		extCred, ok := cred.(*externalCredential)
		if !ok || extCred.connectorURL != nil {
			continue
		}
		if extCred.token == token {
			return extCred
		}
	}
	return nil
}

func (c *externalCredential) connectorLoop() {
	var consecutiveFailures int
	ctx := c.getReverseContext()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sessionLifetime, err := c.connectorConnect(ctx)
		if ctx.Err() != nil {
			return
		}
		if sessionLifetime >= connectorBackoffResetThreshold {
			consecutiveFailures = 0
		}
		consecutiveFailures++
		backoff := connectorBackoff(consecutiveFailures)
		c.logger.Warn("reverse connection for ", c.tag, " lost: ", err, ", reconnecting in ", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
	}
}

const connectorBackoffResetThreshold = time.Minute

func connectorBackoff(failures int) time.Duration {
	if failures > 5 {
		failures = 5
	}
	base := time.Second * time.Duration(1<<failures)
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	jitter := time.Duration(rand.Int64N(int64(base) / 2))
	return base + jitter
}

func (c *externalCredential) connectorConnect(ctx context.Context) (time.Duration, error) {
	if c.reverseService == nil {
		return 0, E.New("reverse service not initialized")
	}
	destination := c.connectorResolveDestination()
	conn, err := c.connectorDialer.DialContext(ctx, "tcp", destination)
	if err != nil {
		return 0, E.Cause(err, "dial")
	}

	if c.connectorTLS != nil {
		tlsConn := stdTLS.Client(conn, c.connectorTLS.Clone())
		err = tlsConn.HandshakeContext(ctx)
		if err != nil {
			conn.Close()
			return 0, E.Cause(err, "tls handshake")
		}
		conn = tlsConn
	}

	upgradeRequest := "GET " + c.connectorRequestPath + " HTTP/1.1\r\n" +
		"Host: " + c.connectorURL.Host + "\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: reverse-proxy\r\n" +
		"Authorization: Bearer " + c.token + "\r\n" +
		"\r\n"
	_, err = io.WriteString(conn, upgradeRequest)
	if err != nil {
		conn.Close()
		return 0, E.Cause(err, "write upgrade request")
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return 0, E.Cause(err, "read upgrade response")
	}
	if !strings.HasPrefix(statusLine, "HTTP/1.1 101") {
		conn.Close()
		return 0, E.New("unexpected upgrade response: ", strings.TrimSpace(statusLine))
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			conn.Close()
			return 0, E.Cause(readErr, "read upgrade headers")
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	session, err := yamux.Server(&bufferedConn{reader: reader, Conn: conn}, reverseYamuxConfig())
	if err != nil {
		conn.Close()
		return 0, E.Cause(err, "create yamux server")
	}
	defer session.Close()

	c.logger.Info("reverse connection established for ", c.tag)

	serveStart := time.Now()
	httpServer := &http.Server{
		Handler:     c.reverseService,
		ReadTimeout: 0,
		IdleTimeout: 120 * time.Second,
	}
	err = httpServer.Serve(&yamuxNetListener{session: session})
	sessionLifetime := time.Since(serveStart)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
		return sessionLifetime, E.Cause(err, "serve")
	}
	return sessionLifetime, E.New("connection closed")
}

func (c *externalCredential) connectorResolveDestination() M.Socksaddr {
	return c.connectorDestination
}
