//go:build windows

package httpclient

import (
	"bufio"
	"context"
	"crypto/sha256"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/proxybridge"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/winhttp"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

const (
	windowsHTTPEngineName = "Windows HTTP engine"
	windowsHTTPUserAgent  = "sing-box/winhttp"
)

var newWindowsProxyBridge = proxybridge.NewHTTP
var windowsHTTPCheckPlatform = winhttp.CheckPlatform

var (
	windowsTransportRequestStates  sync.Map
	windowsTransportRequestStateID atomic.Uint64
	windowsTransportStatusCallback = syscall.NewCallback(windowsTransportStatusProc)
)

type windowsSessionConfig struct {
	version                int
	disableVersionFallback bool
	serverName             string
	minVersion             uint16
	maxVersion             uint16
	insecure               bool
	userRoots              *x509.CertPool
	store                  adapter.CertificateStore
	timeFunc               func() time.Time
	pinnedPublicKeys       [][]byte
	proxyUsername          string
	proxyPassword          string
}

type windowsTransportShared struct {
	logger logger.ContextLogger
	bridge *proxybridge.Bridge
	config windowsSessionConfig
	refs   atomic.Int32
}

type windowsTransport struct {
	shared *windowsTransportShared
	access sync.Mutex
	tree   *windowsTransportTree
	closed bool
}

type windowsTransportTree struct {
	shared *windowsTransportShared

	session     *winhttp.Session
	connectMu   sync.Mutex
	connections map[string]*winhttp.Connection

	active    atomic.Int64
	retired   atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

type windowsRequestState struct {
	id      uintptr
	ctx     context.Context
	request *winhttp.Request

	signalMu sync.Mutex
	sendCh   chan error
	headerCh chan error
	readCh   chan windowsReadResult

	secureFailure atomic.Uint32
	handleClose   sync.Once
	cleanupOnce   sync.Once
	handleClosed  chan struct{}

	sendKeepAliveMu sync.Mutex
	sendHeaders     []uint16
	sendBody        []byte
}

type windowsReadResult struct {
	n   int
	err error
}

type windowsResponseBody struct {
	state   *windowsRequestState
	tree    *windowsTransportTree
	readMu  sync.Mutex
	closeMu sync.Once
}

func newWindowsTransport(ctx context.Context, logger logger.ContextLogger, rawDialer N.Dialer, options option.HTTPClientOptions) (innerTransport, error) {
	config, err := newWindowsSessionConfig(ctx, options)
	if err != nil {
		return nil, err
	}
	bridge, err := newWindowsProxyBridge(ctx, logger, "windows http proxy", rawDialer)
	if err != nil {
		return nil, err
	}
	shared := &windowsTransportShared{
		logger: logger,
		bridge: bridge,
		config: config,
	}
	shared.config.proxyUsername = bridge.Username()
	shared.config.proxyPassword = bridge.Password()
	shared.refs.Store(1)
	tree, err := shared.newTree()
	if err != nil {
		_ = bridge.Close()
		return nil, err
	}
	return &windowsTransport{
		shared: shared,
		tree:   tree,
	}, nil
}

func newWindowsSessionConfig(ctx context.Context, options option.HTTPClientOptions) (windowsSessionConfig, error) {
	err := windowsHTTPCheckPlatform()
	if err != nil {
		return windowsSessionConfig{}, err
	}
	version := options.Version
	if version == 0 {
		version = 2
	}
	switch version {
	case 1, 2:
	case 3:
		return windowsSessionConfig{}, E.New("HTTP/3 is unsupported in Windows HTTP engine")
	default:
		return windowsSessionConfig{}, E.New("unknown HTTP version: ", version)
	}
	if options.HTTP2Options != (option.HTTP2Options{}) {
		return windowsSessionConfig{}, E.New("HTTP/2 options are unsupported in Windows HTTP engine")
	}
	if options.HTTP3Options != (option.QUICOptions{}) {
		return windowsSessionConfig{}, E.New("QUIC options are unsupported in Windows HTTP engine")
	}
	tlsOptions := common.PtrValueOrDefault(options.TLS)
	if tlsOptions.Engine != "" {
		return windowsSessionConfig{}, E.New("tls.engine is unsupported in Windows HTTP engine")
	}
	if len(tlsOptions.ALPN) > 0 {
		return windowsSessionConfig{}, E.New("tls.alpn is unsupported in Windows HTTP engine")
	}
	if tlsOptions.HandshakeTimeout > 0 {
		return windowsSessionConfig{}, E.New("tls.handshake_timeout is unsupported in Windows HTTP engine")
	}
	validated, err := boxTLS.ValidateSystemTLSOptions(ctx, tlsOptions, windowsHTTPEngineName)
	if err != nil {
		return windowsSessionConfig{}, err
	}
	if validated.MinVersion != 0 && validated.MaxVersion != 0 && validated.MinVersion > validated.MaxVersion {
		return windowsSessionConfig{}, E.New("invalid TLS version range")
	}
	var userRoots *x509.CertPool
	if len(validated.UserPEM) > 0 {
		userRoots = x509.NewCertPool()
		if !userRoots.AppendCertsFromPEM(validated.UserPEM) {
			return windowsSessionConfig{}, E.New("parse certificate PEM")
		}
	}
	pins := make([][]byte, 0, len(tlsOptions.CertificatePublicKeySHA256))
	for _, hashValue := range tlsOptions.CertificatePublicKeySHA256 {
		if len(hashValue) != sha256.Size {
			return windowsSessionConfig{}, E.New("invalid certificate_public_key_sha256 length: ", len(hashValue))
		}
		pins = append(pins, append([]byte(nil), hashValue...))
	}
	return windowsSessionConfig{
		version:                version,
		disableVersionFallback: options.DisableVersionFallback,
		serverName:             tlsOptions.ServerName,
		minVersion:             validated.MinVersion,
		maxVersion:             validated.MaxVersion,
		insecure:               tlsOptions.Insecure || len(pins) > 0,
		userRoots:              userRoots,
		store:                  validated.Store,
		timeFunc:               ntp.TimeFuncFromContext(ctx),
		pinnedPublicKeys:       pins,
	}, nil
}

func (s *windowsTransportShared) retain() {
	s.refs.Add(1)
}

func (s *windowsTransportShared) release() error {
	if s.refs.Add(-1) == 0 {
		return s.bridge.Close()
	}
	return nil
}

func (s *windowsTransportShared) newTree() (*windowsTransportTree, error) {
	s.retain()
	releaseShared := true
	defer func() {
		if releaseShared {
			_ = s.release()
		}
	}()
	session, err := winhttp.OpenSession(windowsHTTPUserAgent)
	if err != nil {
		return nil, err
	}
	closeSession := true
	defer func() {
		if closeSession {
			_ = session.Close()
		}
	}()
	err = session.SetStatusCallback(
		windowsTransportStatusCallback,
		winhttp.CallbackFlagHandles|
			winhttp.CallbackFlagSendRequestComplete|
			winhttp.CallbackFlagHeadersAvailable|
			winhttp.CallbackFlagReadComplete|
			winhttp.CallbackFlagRequestError|
			winhttp.CallbackFlagSecureFailure,
	)
	if err != nil {
		return nil, err
	}
	err = session.SetProxy(net.JoinHostPort("127.0.0.1", strconv.Itoa(int(s.bridge.Port()))))
	if err != nil {
		return nil, err
	}
	err = session.DisableGlobalPooling()
	if err != nil {
		return nil, err
	}
	err = session.SetSecureProtocols(windowsSecureProtocols(s.config.minVersion, s.config.maxVersion))
	if err != nil {
		return nil, err
	}
	closeSession = false
	releaseShared = false
	return &windowsTransportTree{
		shared:      s,
		session:     session,
		connections: make(map[string]*winhttp.Connection),
	}, nil
}

func (t *windowsTransportTree) retain() {
	t.active.Add(1)
}

func (t *windowsTransportTree) release() {
	if t.active.Add(-1) == 0 && t.retired.Load() {
		t.close()
	}
}

func (t *windowsTransportTree) retire() {
	t.retired.Store(true)
	if t.active.Load() == 0 {
		t.close()
	}
}

func (t *windowsTransportTree) close() {
	t.closeOnce.Do(func() {
		t.connectMu.Lock()
		connections := make([]*winhttp.Connection, 0, len(t.connections))
		for _, connection := range t.connections {
			connections = append(connections, connection)
		}
		t.connections = nil
		t.connectMu.Unlock()
		var errs []error
		for _, connection := range connections {
			err := connection.Close()
			if err != nil {
				errs = append(errs, err)
			}
		}
		err := t.session.Close()
		if err != nil {
			errs = append(errs, err)
		}
		err = t.shared.release()
		if err != nil {
			errs = append(errs, err)
		}
		t.closeErr = E.Errors(errs...)
	})
}

func (t *windowsTransportTree) connection(authority string) (*winhttp.Connection, error) {
	t.connectMu.Lock()
	defer t.connectMu.Unlock()
	if connection, loaded := t.connections[authority]; loaded {
		return connection, nil
	}
	host, portString, err := net.SplitHostPort(authority)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return nil, err
	}
	connection, err := t.session.Connect(host, uint16(port))
	if err != nil {
		return nil, err
	}
	t.connections[authority] = connection
	return connection, nil
}

func (t *windowsTransport) acquireTree() (*windowsTransportTree, error) {
	t.access.Lock()
	defer t.access.Unlock()
	if t.closed {
		return nil, net.ErrClosed
	}
	tree := t.tree
	if tree == nil {
		var err error
		tree, err = t.shared.newTree()
		if err != nil {
			return nil, err
		}
		t.tree = tree
	}
	tree.retain()
	return tree, nil
}

func (t *windowsTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if requestRequiresHTTP1(request) {
		return nil, E.New("HTTP upgrade requests are unsupported in Windows HTTP engine")
	}
	if request.URL == nil {
		return nil, E.New("missing request URL")
	}
	switch request.URL.Scheme {
	case "http", "https":
	default:
		return nil, E.New("unsupported URL scheme: ", request.URL.Scheme)
	}
	authority := requestAuthority(request)
	if authority == "" {
		return nil, E.New("missing request authority")
	}
	authorityHost, _, _ := net.SplitHostPort(authority)
	verifyName := authorityHost
	if request.URL.Scheme == "https" && t.shared.config.serverName != "" {
		if !strings.EqualFold(t.shared.config.serverName, request.URL.Hostname()) {
			return nil, E.New("tls.server_name is unsupported in Windows HTTP engine unless it matches request host")
		}
		verifyName = t.shared.config.serverName
	}
	var body []byte
	if request.Body != nil && request.Body != http.NoBody {
		defer request.Body.Close()
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
	}
	tree, err := t.acquireTree()
	if err != nil {
		return nil, err
	}
	connection, err := tree.connection(authority)
	if err != nil {
		tree.release()
		return nil, err
	}
	requestHandle, err := connection.OpenRequest(request.Method, windowsRequestObjectName(request.URL), request.URL.Scheme == "https")
	if err != nil {
		tree.release()
		return nil, err
	}
	state := newWindowsRequestState(request.Context(), requestHandle)
	defer func() {
		if err != nil {
			state.clearSendKeepAlive()
			state.closeHandle()
			tree.release()
		}
	}()
	err = requestHandle.SetContextValue(state.id)
	if err != nil {
		return nil, err
	}
	err = configureWindowsRequest(requestHandle, t.shared.config, request.URL.Scheme == "https")
	if err != nil {
		return nil, err
	}
	sendHeaders := windowsRequestHeadersUTF16(request)
	state.setSendKeepAlive(sendHeaders, body)
	sendCh := state.armSend()
	err = requestHandle.Send(sendHeaders, body)
	if err != nil {
		state.clearSend(sendCh)
		state.clearSendKeepAlive()
		return nil, err
	}
	err = state.wait(sendCh)
	state.clearSendKeepAlive()
	if err != nil {
		return nil, err
	}
	headerCh := state.armHeaders()
	err = requestHandle.ReceiveResponse()
	if err != nil {
		state.clearHeaders(headerCh)
		return nil, err
	}
	err = state.wait(headerCh)
	if err != nil {
		return nil, err
	}
	rawHeaders, err := requestHandle.QueryRawHeadersCRLF()
	if err != nil {
		return nil, err
	}
	response, err := http.ReadResponse(bufio.NewReader(strings.NewReader(rawHeaders)), request)
	if err != nil {
		return nil, E.Cause(err, "parse response headers")
	}
	protocolUsed, protocolErr := requestHandle.QueryHTTPProtocolUsed()
	if protocolErr == nil {
		response.Proto, response.ProtoMajor, response.ProtoMinor = windowsResponseProtocol(protocolUsed, response.Proto)
	}
	if request.URL.Scheme == "https" {
		connectionState, tlsErr := buildWindowsResponseTLSState(t.shared.config, requestHandle, verifyName, response.Proto)
		if tlsErr != nil {
			return nil, tlsErr
		}
		response.TLS = connectionState
	}
	response.Request = request
	response.Body = &windowsResponseBody{
		state: state,
		tree:  tree,
	}
	return response, nil
}

func (t *windowsTransport) CloseIdleConnections() {
	newTree, err := t.shared.newTree()
	if err != nil {
		t.shared.logger.Error(E.Cause(err, "reset Windows HTTP session"))
		return
	}
	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		newTree.retire()
		return
	}
	oldTree := t.tree
	t.tree = newTree
	t.access.Unlock()
	if oldTree != nil {
		oldTree.retire()
	}
}

func (t *windowsTransport) Close() error {
	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		return nil
	}
	t.closed = true
	tree := t.tree
	t.tree = nil
	t.access.Unlock()
	if tree != nil {
		tree.retire()
	}
	return t.shared.release()
}

func configureWindowsRequest(requestHandle *winhttp.Request, config windowsSessionConfig, secure bool) error {
	err := requestHandle.SetProxyCredentials(config.proxyUsername, config.proxyPassword)
	if err != nil {
		return err
	}
	err = requestHandle.SetRedirectPolicy(winhttp.RedirectPolicyNever)
	if err != nil {
		return err
	}
	err = requestHandle.SetDisableFeature(winhttp.DisableRedirects | winhttp.DisableCookies)
	if err != nil {
		return err
	}
	if !secure {
		return applyWindowsHTTPVersion(requestHandle, config)
	}
	securityFlags := windowsSecurityFlags(config)
	if securityFlags != 0 {
		err = requestHandle.SetSecurityFlags(securityFlags)
		if err != nil {
			return err
		}
	}
	return applyWindowsHTTPVersion(requestHandle, config)
}

func applyWindowsHTTPVersion(requestHandle *winhttp.Request, config windowsSessionConfig) error {
	switch config.version {
	case 1:
		return requestHandle.SetEnableHTTPProtocol(0)
	case 2:
		err := requestHandle.SetEnableHTTPProtocol(winhttp.ProtocolFlagHTTP2)
		if err != nil {
			return err
		}
		if config.disableVersionFallback {
			return requestHandle.SetRequiredHTTPProtocol(winhttp.ProtocolFlagHTTP2)
		}
		return nil
	default:
		return E.New("unknown HTTP version: ", config.version)
	}
}

func windowsSecureProtocols(minVersion, maxVersion uint16) uint32 {
	versions := []struct {
		version uint16
		mask    uint32
	}{
		{stdtls.VersionTLS10, winhttp.SecureProtocolTLS10},
		{stdtls.VersionTLS11, winhttp.SecureProtocolTLS11},
		{stdtls.VersionTLS12, winhttp.SecureProtocolTLS12},
		{stdtls.VersionTLS13, winhttp.SecureProtocolTLS13},
	}
	effectiveMin := minVersion
	if effectiveMin == 0 {
		effectiveMin = stdtls.VersionTLS12
	}
	effectiveMax := maxVersion
	if effectiveMax == 0 {
		effectiveMax = stdtls.VersionTLS13
	}
	var mask uint32
	for _, version := range versions {
		if version.version >= effectiveMin && version.version <= effectiveMax {
			mask |= version.mask
		}
	}
	return mask
}

func windowsSecurityFlags(config windowsSessionConfig) uint32 {
	switch {
	case config.insecure || len(config.pinnedPublicKeys) > 0:
		return winhttp.SecurityFlagIgnoreUnknownCA |
			winhttp.SecurityFlagIgnoreCertCNInvalid |
			winhttp.SecurityFlagIgnoreCertDateInvalid |
			winhttp.SecurityFlagIgnoreCertWrongUsage
	case config.userRoots != nil || config.store != nil:
		return winhttp.SecurityFlagIgnoreUnknownCA
	default:
		return 0
	}
}

func windowsRequestObjectName(rawURL *url.URL) string {
	objectName := rawURL.EscapedPath()
	if objectName == "" {
		objectName = "/"
	}
	if rawURL.RawQuery != "" {
		objectName += "?" + rawURL.RawQuery
	}
	return objectName
}

func windowsRequestHeadersUTF16(request *http.Request) []uint16 {
	var builder strings.Builder
	for key, values := range request.Header {
		for _, value := range values {
			builder.WriteString(key)
			builder.WriteString(": ")
			builder.WriteString(value)
			builder.WriteString("\r\n")
		}
	}
	if request.Host != "" {
		builder.WriteString("Host: ")
		builder.WriteString(request.Host)
		builder.WriteString("\r\n")
	}
	if builder.Len() == 0 {
		return nil
	}
	return syscall.StringToUTF16(builder.String())
}

func buildWindowsResponseTLSState(config windowsSessionConfig, requestHandle *winhttp.Request, serverName string, responseProto string) (*stdtls.ConnectionState, error) {
	rawCerts, err := requestHandle.QueryServerCertificateChainDER()
	if err != nil {
		return nil, err
	}
	peerCertificates := make([]*x509.Certificate, 0, len(rawCerts))
	for index, certDER := range rawCerts {
		cert, parseErr := x509.ParseCertificate(certDER)
		if parseErr != nil {
			return nil, E.Cause(parseErr, "parse peer certificate ", index)
		}
		peerCertificates = append(peerCertificates, cert)
	}
	err = verifyWindowsResponseCertificates(config, serverName, peerCertificates, rawCerts)
	if err != nil {
		return nil, err
	}
	state := &stdtls.ConnectionState{
		HandshakeComplete: true,
		ServerName:        serverName,
		PeerCertificates:  peerCertificates,
	}
	if responseProto == "HTTP/2.0" {
		state.NegotiatedProtocol = "h2"
	}
	securityInfo, securityErr := requestHandle.QuerySecurityInfo()
	if securityErr == nil {
		state.Version = windowsTLSVersionFromProtocol(securityInfo.ConnectionInfo.Protocol)
		state.CipherSuite = uint16(securityInfo.CipherInfo.CipherSuite)
	}
	return state, nil
}

func verifyWindowsResponseCertificates(config windowsSessionConfig, serverName string, peerCertificates []*x509.Certificate, rawCerts [][]byte) error {
	if !config.insecure {
		var roots *x509.CertPool
		switch {
		case config.userRoots != nil:
			roots = config.userRoots
		case config.store != nil:
			roots = config.store.Pool()
		}
		err := verifyWindowsPeerCertificates(roots, serverName, config.timeFunc, peerCertificates)
		if err != nil {
			return err
		}
	}
	if len(config.pinnedPublicKeys) > 0 {
		return boxTLS.VerifyPublicKeySHA256(config.pinnedPublicKeys, rawCerts)
	}
	return nil
}

func verifyWindowsPeerCertificates(roots *x509.CertPool, serverName string, timeFunc func() time.Time, peerCertificates []*x509.Certificate) error {
	if len(peerCertificates) == 0 {
		return E.New("no peer certificates")
	}
	intermediates := x509.NewCertPool()
	for _, cert := range peerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	verifyOptions := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       serverName,
	}
	if timeFunc != nil {
		verifyOptions.CurrentTime = timeFunc()
	}
	_, err := peerCertificates[0].Verify(verifyOptions)
	return err
}

func windowsTLSVersionFromProtocol(protocol uint32) uint16 {
	switch {
	case protocol&0x00002000 != 0:
		return stdtls.VersionTLS13
	case protocol&0x00000800 != 0:
		return stdtls.VersionTLS12
	case protocol&0x00000200 != 0:
		return stdtls.VersionTLS11
	case protocol&0x00000080 != 0:
		return stdtls.VersionTLS10
	default:
		return 0
	}
}

func windowsResponseProtocol(protocolUsed uint32, fallback string) (string, int, int) {
	switch {
	case protocolUsed&winhttp.ProtocolFlagHTTP2 != 0:
		return "HTTP/2.0", 2, 0
	case protocolUsed&winhttp.ProtocolFlagHTTP3 != 0:
		return "HTTP/3.0", 3, 0
	default:
		if major, minor, ok := http.ParseHTTPVersion(fallback); ok {
			return fallback, major, minor
		}
		return "HTTP/1.1", 1, 1
	}
}

func newWindowsRequestState(ctx context.Context, request *winhttp.Request) *windowsRequestState {
	id := uintptr(windowsTransportRequestStateID.Add(1))
	state := &windowsRequestState{
		id:           id,
		ctx:          ctx,
		request:      request,
		handleClosed: make(chan struct{}),
	}
	windowsTransportRequestStates.Store(id, state)
	go func() {
		select {
		case <-ctx.Done():
			state.closeHandle()
		case <-state.handleClosed:
		}
	}()
	return state
}

func (s *windowsRequestState) armSend() chan error {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	ch := make(chan error, 1)
	s.sendCh = ch
	return ch
}

func (s *windowsRequestState) clearSend(ch chan error) {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	if s.sendCh == ch {
		s.sendCh = nil
	}
}

func (s *windowsRequestState) armHeaders() chan error {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	ch := make(chan error, 1)
	s.headerCh = ch
	return ch
}

func (s *windowsRequestState) clearHeaders(ch chan error) {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	if s.headerCh == ch {
		s.headerCh = nil
	}
}

func (s *windowsRequestState) armRead() chan windowsReadResult {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	ch := make(chan windowsReadResult, 1)
	s.readCh = ch
	return ch
}

func (s *windowsRequestState) clearRead(ch chan windowsReadResult) {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	if s.readCh == ch {
		s.readCh = nil
	}
}

func (s *windowsRequestState) wait(ch chan error) error {
	err := <-ch
	return s.mapError(err)
}

func (s *windowsRequestState) setSendKeepAlive(headers []uint16, body []byte) {
	s.sendKeepAliveMu.Lock()
	s.sendHeaders = headers
	s.sendBody = body
	s.sendKeepAliveMu.Unlock()
}

func (s *windowsRequestState) clearSendKeepAlive() {
	s.sendKeepAliveMu.Lock()
	s.sendHeaders = nil
	s.sendBody = nil
	s.sendKeepAliveMu.Unlock()
}

func (s *windowsRequestState) signalSend(err error) {
	s.signalMu.Lock()
	ch := s.sendCh
	s.sendCh = nil
	s.signalMu.Unlock()
	if ch != nil {
		ch <- err
	}
}

func (s *windowsRequestState) signalHeaders(err error) {
	s.signalMu.Lock()
	ch := s.headerCh
	s.headerCh = nil
	s.signalMu.Unlock()
	if ch != nil {
		ch <- err
	}
}

func (s *windowsRequestState) signalRead(result windowsReadResult) {
	s.signalMu.Lock()
	ch := s.readCh
	s.readCh = nil
	s.signalMu.Unlock()
	if ch != nil {
		ch <- result
	}
}

func (s *windowsRequestState) signalAny(err error) {
	s.signalMu.Lock()
	sendCh := s.sendCh
	headerCh := s.headerCh
	readCh := s.readCh
	s.sendCh = nil
	s.headerCh = nil
	s.readCh = nil
	s.signalMu.Unlock()
	switch {
	case sendCh != nil:
		sendCh <- err
	case headerCh != nil:
		headerCh <- err
	case readCh != nil:
		readCh <- windowsReadResult{err: err}
	}
}

func (s *windowsRequestState) closeHandle() error {
	var closeErr error
	s.handleClose.Do(func() {
		closeErr = s.request.Close()
	})
	return closeErr
}

func (s *windowsRequestState) cleanup() {
	s.cleanupOnce.Do(func() {
		windowsTransportRequestStates.Delete(s.id)
		close(s.handleClosed)
	})
}

func (s *windowsRequestState) mapError(err error) error {
	if err == nil {
		return nil
	}
	if s.ctx.Err() != nil && (errors.Is(err, winhttp.ErrorOperationCancelled) || errors.Is(err, net.ErrClosed)) {
		return s.ctx.Err()
	}
	return err
}

func (b *windowsResponseBody) Read(p []byte) (int, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	readCh := b.state.armRead()
	err := b.state.request.ReadData(p)
	if err != nil {
		b.state.clearRead(readCh)
		return 0, b.state.mapError(err)
	}
	result := <-readCh
	runtime.KeepAlive(p)
	if result.err != nil {
		b.state.closeHandle()
		return 0, b.state.mapError(result.err)
	}
	if result.n == 0 {
		b.state.closeHandle()
		return 0, io.EOF
	}
	return result.n, nil
}

func (b *windowsResponseBody) Close() error {
	var closeErr error
	b.closeMu.Do(func() {
		closeErr = b.state.closeHandle()
		b.tree.release()
	})
	return closeErr
}

func windowsTransportStatusProc(_ uintptr, context uintptr, status uint32, info uintptr, infoLen uint32) uintptr {
	stateValue, loaded := windowsTransportRequestStates.Load(context)
	if !loaded {
		return 0
	}
	state := stateValue.(*windowsRequestState)
	switch status {
	case winhttp.CallbackStatusSendRequestComplete:
		state.signalSend(nil)
	case winhttp.CallbackStatusHeadersAvailable:
		state.signalHeaders(nil)
	case winhttp.CallbackStatusReadComplete:
		state.signalRead(windowsReadResult{n: int(infoLen)})
	case winhttp.CallbackStatusRequestError:
		asyncResult := (*winhttp.AsyncResult)(unsafe.Pointer(info))
		asyncErr := error(winhttp.Error(asyncResult.Error))
		switch asyncResult.Result {
		case winhttp.APISendRequest:
			state.signalSend(asyncErr)
		case winhttp.APIReceiveResponse:
			state.signalHeaders(asyncErr)
		case winhttp.APIReadData:
			state.signalRead(windowsReadResult{err: asyncErr})
		default:
			state.signalAny(asyncErr)
		}
	case winhttp.CallbackStatusSecureFailure:
		if info != 0 && infoLen >= uint32(unsafe.Sizeof(uint32(0))) {
			state.secureFailure.Store(*(*uint32)(unsafe.Pointer(info)))
		}
	case winhttp.CallbackStatusHandleClosing:
		state.cleanup()
	}
	return 0
}
