//go:build darwin && cgo

package httpclient

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework Security

#include <stdlib.h>
#include "apple_transport_darwin.h"
*/
import "C"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/certificate"
	"github.com/sagernet/sing-box/common/proxybridge"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

const applePinnedHashSize = sha256.Size

var (
	newAppleUserAnchors      = certificate.NewAppleAnchors
	newAppleProxyBridge      = proxybridge.New
	newAppleTransportSession = func(shared *appleTransportShared) (unsafe.Pointer, error) {
		session, err := shared.newSession()
		return unsafe.Pointer(session), err
	}
)

func verifyApplePinnedPublicKeySHA256(flatHashes []byte, leafCertificate []byte) error {
	if len(flatHashes)%applePinnedHashSize != 0 {
		return E.New("invalid pinned public key list")
	}
	knownHashes := make([][]byte, 0, len(flatHashes)/applePinnedHashSize)
	for offset := 0; offset < len(flatHashes); offset += applePinnedHashSize {
		knownHashes = append(knownHashes, append([]byte(nil), flatHashes[offset:offset+applePinnedHashSize]...))
	}
	return boxTLS.VerifyPublicKeySHA256(knownHashes, [][]byte{leafCertificate})
}

//export box_apple_http_verify_public_key_sha256
func box_apple_http_verify_public_key_sha256(knownHashValues *C.uint8_t, knownHashValuesLen C.size_t, leafCert *C.uint8_t, leafCertLen C.size_t) *C.char {
	flatHashes := C.GoBytes(unsafe.Pointer(knownHashValues), C.int(knownHashValuesLen))
	leafCertificate := C.GoBytes(unsafe.Pointer(leafCert), C.int(leafCertLen))
	err := verifyApplePinnedPublicKeySHA256(flatHashes, leafCertificate)
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}

type appleSessionConfig struct {
	serverName             string
	minVersion             uint16
	maxVersion             uint16
	insecure               bool
	anchorOnly             bool
	userAnchors            adapter.AppleAnchors
	store                  adapter.CertificateStore
	pinnedPublicKeySHA256s []byte
}

type appleTransportShared struct {
	logger   logger.ContextLogger
	bridge   *proxybridge.Bridge
	config   appleSessionConfig
	timeFunc func() time.Time
	refs     atomic.Int32
}

type appleTransport struct {
	shared  *appleTransportShared
	access  sync.Mutex
	session *C.box_apple_http_session_t
	closed  bool
}

func newAppleTransport(ctx context.Context, logger logger.ContextLogger, rawDialer N.Dialer, options option.HTTPClientOptions) (innerTransport, error) {
	sessionConfig, err := newAppleSessionConfig(ctx, options)
	if err != nil {
		return nil, err
	}
	releaseConfig := true
	defer func() {
		if releaseConfig {
			sessionConfig.close()
		}
	}()
	bridge, err := newAppleProxyBridge(ctx, logger, "apple http proxy", rawDialer)
	if err != nil {
		return nil, err
	}
	shared := &appleTransportShared{
		logger:   logger,
		bridge:   bridge,
		config:   sessionConfig,
		timeFunc: ntp.TimeFuncFromContext(ctx),
	}
	shared.refs.Store(1)
	sessionRef, err := newAppleTransportSession(shared)
	if err != nil {
		bridge.Close()
		return nil, err
	}
	session := (*C.box_apple_http_session_t)(sessionRef)
	releaseConfig = false
	return &appleTransport{
		shared:  shared,
		session: session,
	}, nil
}

func newAppleSessionConfig(ctx context.Context, options option.HTTPClientOptions) (appleSessionConfig, error) {
	version := options.Version
	if version == 0 {
		version = 2
	}
	switch version {
	case 2:
	case 1:
		return appleSessionConfig{}, E.New("HTTP/1.1 is unsupported in Apple HTTP engine")
	case 3:
		return appleSessionConfig{}, E.New("HTTP/3 is unsupported in Apple HTTP engine")
	default:
		return appleSessionConfig{}, E.New("unknown HTTP version: ", version)
	}
	if options.DisableVersionFallback {
		return appleSessionConfig{}, E.New("disable_version_fallback is unsupported in Apple HTTP engine")
	}
	if options.HTTP2Options != (option.HTTP2Options{}) {
		return appleSessionConfig{}, E.New("HTTP/2 options are unsupported in Apple HTTP engine")
	}
	if options.HTTP3Options != (option.QUICOptions{}) {
		return appleSessionConfig{}, E.New("QUIC options are unsupported in Apple HTTP engine")
	}

	tlsOptions := common.PtrValueOrDefault(options.TLS)
	if tlsOptions.Engine != "" {
		return appleSessionConfig{}, E.New("tls.engine is unsupported in Apple HTTP engine")
	}
	if len(tlsOptions.ALPN) > 0 {
		return appleSessionConfig{}, E.New("tls.alpn is unsupported in Apple HTTP engine")
	}
	validated, err := boxTLS.ValidateSystemTLSOptions(ctx, tlsOptions, "Apple HTTP engine")
	if err != nil {
		return appleSessionConfig{}, err
	}

	config := appleSessionConfig{
		serverName: tlsOptions.ServerName,
		minVersion: validated.MinVersion,
		maxVersion: validated.MaxVersion,
		insecure:   tlsOptions.Insecure || len(tlsOptions.CertificatePublicKeySHA256) > 0,
		anchorOnly: validated.Exclusive,
		store:      validated.Store,
	}
	if len(validated.UserPEM) > 0 {
		userAnchors, anchorsErr := newAppleUserAnchors(validated.UserPEM)
		if anchorsErr != nil {
			return appleSessionConfig{}, anchorsErr
		}
		config.userAnchors = userAnchors
	}
	if len(tlsOptions.CertificatePublicKeySHA256) > 0 {
		config.pinnedPublicKeySHA256s = make([]byte, 0, len(tlsOptions.CertificatePublicKeySHA256)*applePinnedHashSize)
		for _, hashValue := range tlsOptions.CertificatePublicKeySHA256 {
			if len(hashValue) != applePinnedHashSize {
				if config.userAnchors != nil {
					config.userAnchors.Release()
				}
				return appleSessionConfig{}, E.New("invalid certificate_public_key_sha256 length: ", len(hashValue))
			}
			config.pinnedPublicKeySHA256s = append(config.pinnedPublicKeySHA256s, hashValue...)
		}
	}
	return config, nil
}

func (c *appleSessionConfig) close() {
	if c.userAnchors != nil {
		c.userAnchors.Release()
		c.userAnchors = nil
	}
}

func (s *appleTransportShared) retain() {
	s.refs.Add(1)
}

func (s *appleTransportShared) release() error {
	if s.refs.Add(-1) == 0 {
		s.config.close()
		return s.bridge.Close()
	}
	return nil
}

func (s *appleTransportShared) newSession() (*C.box_apple_http_session_t, error) {
	cProxyHost := C.CString("127.0.0.1")
	defer C.free(unsafe.Pointer(cProxyHost))
	cProxyUsername := C.CString(s.bridge.Username())
	defer C.free(unsafe.Pointer(cProxyUsername))
	cProxyPassword := C.CString(s.bridge.Password())
	defer C.free(unsafe.Pointer(cProxyPassword))
	var pinnedPointer *C.uint8_t
	if len(s.config.pinnedPublicKeySHA256s) > 0 {
		pinnedPointer = (*C.uint8_t)(C.CBytes(s.config.pinnedPublicKeySHA256s))
		defer C.free(unsafe.Pointer(pinnedPointer))
	}
	anchors := certificate.AcquireAnchors(s.config.userAnchors, s.config.store)
	var anchorsRef unsafe.Pointer
	if anchors != nil {
		anchorsRef = anchors.Ref()
		defer anchors.Release()
	}
	cConfig := C.box_apple_http_session_config_t{
		proxy_host:                   cProxyHost,
		proxy_port:                   C.int(s.bridge.Port()),
		proxy_username:               cProxyUsername,
		proxy_password:               cProxyPassword,
		min_tls_version:              C.uint16_t(s.config.minVersion),
		max_tls_version:              C.uint16_t(s.config.maxVersion),
		insecure:                     C.bool(s.config.insecure),
		anchors_cf:                   anchorsRef,
		anchor_only:                  C.bool(s.config.anchorOnly),
		pinned_public_key_sha256:     pinnedPointer,
		pinned_public_key_sha256_len: C.size_t(len(s.config.pinnedPublicKeySHA256s)),
	}
	var cErr *C.char
	session := C.box_apple_http_session_create(&cConfig, &cErr)
	if session != nil {
		return session, nil
	}
	return nil, appleCStringError(cErr, "create Apple HTTP session")
}

func (t *appleTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if requestRequiresHTTP1(request) {
		return nil, E.New("HTTP upgrade requests are unsupported in Apple HTTP engine")
	}
	if request.URL == nil {
		return nil, E.New("missing request URL")
	}
	switch request.URL.Scheme {
	case "http", "https":
	default:
		return nil, E.New("unsupported URL scheme: ", request.URL.Scheme)
	}
	if request.URL.Scheme == "https" && t.shared.config.serverName != "" && !strings.EqualFold(t.shared.config.serverName, request.URL.Hostname()) {
		return nil, E.New("tls.server_name is unsupported in Apple HTTP engine unless it matches request host")
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
	headerKeys, headerValues := flattenRequestHeaders(request)
	cMethod := C.CString(request.Method)
	defer C.free(unsafe.Pointer(cMethod))
	cURL := C.CString(request.URL.String())
	defer C.free(unsafe.Pointer(cURL))
	cHeaderKeys := make([]*C.char, len(headerKeys))
	cHeaderValues := make([]*C.char, len(headerValues))
	defer func() {
		for _, ptr := range cHeaderKeys {
			C.free(unsafe.Pointer(ptr))
		}
		for _, ptr := range cHeaderValues {
			C.free(unsafe.Pointer(ptr))
		}
	}()
	for index, value := range headerKeys {
		cHeaderKeys[index] = C.CString(value)
	}
	for index, value := range headerValues {
		cHeaderValues[index] = C.CString(value)
	}
	var headerKeysPointer **C.char
	var headerValuesPointer **C.char
	if len(cHeaderKeys) > 0 {
		pointerArraySize := C.size_t(len(cHeaderKeys)) * C.size_t(unsafe.Sizeof((*C.char)(nil)))
		headerKeysPointer = (**C.char)(C.malloc(pointerArraySize))
		defer C.free(unsafe.Pointer(headerKeysPointer))
		headerValuesPointer = (**C.char)(C.malloc(pointerArraySize))
		defer C.free(unsafe.Pointer(headerValuesPointer))
		copy(unsafe.Slice(headerKeysPointer, len(cHeaderKeys)), cHeaderKeys)
		copy(unsafe.Slice(headerValuesPointer, len(cHeaderValues)), cHeaderValues)
	}
	var bodyPointer *C.uint8_t
	if len(body) > 0 {
		bodyPointer = (*C.uint8_t)(C.CBytes(body))
		defer C.free(unsafe.Pointer(bodyPointer))
	}
	var (
		hasVerifyTime       bool
		verifyTimeUnixMilli int64
	)
	if t.shared.timeFunc != nil {
		hasVerifyTime = true
		verifyTimeUnixMilli = t.shared.timeFunc().UnixMilli()
	}
	cRequest := C.box_apple_http_request_t{
		method:                  cMethod,
		url:                     cURL,
		header_keys:             (**C.char)(headerKeysPointer),
		header_values:           (**C.char)(headerValuesPointer),
		header_count:            C.size_t(len(cHeaderKeys)),
		body:                    bodyPointer,
		body_len:                C.size_t(len(body)),
		has_verify_time:         C.bool(hasVerifyTime),
		verify_time_unix_millis: C.int64_t(verifyTimeUnixMilli),
	}
	var cErr *C.char
	var task *C.box_apple_http_task_t
	t.access.Lock()
	if t.session == nil {
		t.access.Unlock()
		return nil, net.ErrClosed
	}
	// Keep the session attached until NSURLSession has created the task.
	task = C.box_apple_http_session_send_async(t.session, &cRequest, &cErr)
	t.access.Unlock()
	if task == nil {
		return nil, appleCStringError(cErr, "create Apple HTTP request")
	}
	cancelDone := make(chan struct{})
	cancelExit := make(chan struct{})
	go func() {
		defer close(cancelExit)
		select {
		case <-request.Context().Done():
			C.box_apple_http_task_cancel(task)
		case <-cancelDone:
		}
	}()
	cResponse := C.box_apple_http_task_wait(task, &cErr)
	close(cancelDone)
	<-cancelExit
	C.box_apple_http_task_close(task)
	if cResponse == nil {
		err := appleCStringError(cErr, "Apple HTTP request failed")
		if request.Context().Err() != nil {
			return nil, request.Context().Err()
		}
		return nil, err
	}
	defer C.box_apple_http_response_free(cResponse)
	return parseAppleHTTPResponse(request, cResponse), nil
}

func (t *appleTransport) CloseIdleConnections() {
	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		return
	}
	t.access.Unlock()
	newSession, err := t.shared.newSession()
	if err != nil {
		t.shared.logger.Error(E.Cause(err, "reset Apple HTTP session"))
		return
	}
	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		C.box_apple_http_session_close(newSession)
		return
	}
	oldSession := t.session
	t.session = newSession
	t.access.Unlock()
	C.box_apple_http_session_retire(oldSession)
}

func (t *appleTransport) Close() error {
	t.access.Lock()
	if t.closed {
		t.access.Unlock()
		return nil
	}
	t.closed = true
	session := t.session
	t.session = nil
	t.access.Unlock()
	C.box_apple_http_session_close(session)
	return t.shared.release()
}

func flattenRequestHeaders(request *http.Request) ([]string, []string) {
	var (
		keys   []string
		values []string
	)
	for key, headerValues := range request.Header {
		for _, value := range headerValues {
			keys = append(keys, key)
			values = append(values, value)
		}
	}
	if request.Host != "" {
		keys = append(keys, "Host")
		values = append(values, request.Host)
	}
	return keys, values
}

func parseAppleHTTPResponse(request *http.Request, response *C.box_apple_http_response_t) *http.Response {
	headers := make(http.Header)
	headerKeys := unsafe.Slice(response.header_keys, int(response.header_count))
	headerValues := unsafe.Slice(response.header_values, int(response.header_count))
	for index := range headerKeys {
		headers.Add(C.GoString(headerKeys[index]), C.GoString(headerValues[index]))
	}
	body := bytes.NewReader(C.GoBytes(unsafe.Pointer(response.body), C.int(response.body_len)))
	// NSURLSession's completion-handler API does not expose the negotiated protocol;
	// callers that read Response.Proto will see HTTP/1.1 even when the wire was HTTP/2.
	return &http.Response{
		StatusCode:    int(response.status_code),
		Status:        fmt.Sprintf("%d %s", int(response.status_code), http.StatusText(int(response.status_code))),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        headers,
		Body:          io.NopCloser(body),
		ContentLength: int64(body.Len()),
		Request:       request,
	}
}

func appleCStringError(cErr *C.char, message string) error {
	if cErr == nil {
		return E.New(message)
	}
	defer C.free(unsafe.Pointer(cErr))
	return E.New(message, ": ", C.GoString(cErr))
}
