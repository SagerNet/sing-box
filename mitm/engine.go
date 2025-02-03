package mitm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	sTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
	sHTTP "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/service"

	"golang.org/x/crypto/pkcs12"
	"golang.org/x/net/http2"
)

var _ adapter.MITMEngine = (*Engine)(nil)

type Engine struct {
	ctx        context.Context
	logger     logger.ContextLogger
	connection adapter.ConnectionManager
	script     adapter.ScriptManager
	timeFunc   func() time.Time

	http2Enabled         bool
	tlsDecryptionEnabled bool
	tlsPrivateKey        any
	tlsCertificate       *x509.Certificate
}

func NewEngine(ctx context.Context, logger logger.ContextLogger, options option.MITMOptions) (*Engine, error) {
	engine := &Engine{
		ctx:          ctx,
		logger:       logger,
		http2Enabled: options.HTTP2Enabled,
	}
	if options.TLSDecryptionOptions != nil && options.TLSDecryptionOptions.Enabled {
		pfxBytes, err := base64.StdEncoding.DecodeString(options.TLSDecryptionOptions.KeyPair)
		if err != nil {
			return nil, E.Cause(err, "decode key pair base64 bytes")
		}
		privateKey, certificate, err := pkcs12.Decode(pfxBytes, options.TLSDecryptionOptions.KeyPassword)
		if err != nil {
			return nil, E.Cause(err, "decode key pair")
		}
		engine.tlsDecryptionEnabled = true
		engine.tlsPrivateKey = privateKey
		engine.tlsCertificate = certificate
	}
	return engine, nil
}

func (e *Engine) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		e.connection = service.FromContext[adapter.ConnectionManager](e.ctx)
		e.script = service.FromContext[adapter.ScriptManager](e.ctx)
		e.timeFunc = ntp.TimeFuncFromContext(e.ctx)
		if e.timeFunc == nil {
			e.timeFunc = time.Now
		}
	}
	return nil
}

func (e *Engine) Close() error {
	return nil
}

func (e *Engine) ExportCertificate() *x509.Certificate {
	return e.tlsCertificate
}

func (e *Engine) NewConnection(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if e.tlsDecryptionEnabled && metadata.ClientHello != nil {
		err := e.newTLS(ctx, this, conn, metadata, onClose)
		if err != nil {
			e.logger.ErrorContext(ctx, err)
		} else {
			e.logger.DebugContext(ctx, "connection closed")
		}
		if onClose != nil {
			onClose(err)
		}
		return
	} else if metadata.HTTPRequest != nil {
		err := e.newHTTP1(ctx, this, conn, nil, metadata)
		if err != nil {
			e.logger.ErrorContext(ctx, err)
		} else {
			e.logger.DebugContext(ctx, "connection closed")
		}
		if onClose != nil {
			onClose(err)
		}
		return
	} else {
		e.logger.DebugContext(ctx, "HTTP and TLS not detected, skipped")
	}
	metadata.MITM = nil
	e.connection.NewConnection(ctx, this, conn, metadata, onClose)
}

func (e *Engine) newTLS(ctx context.Context, this N.Dialer, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	acceptHTTP := len(metadata.ClientHello.SupportedProtos) == 0 || common.Contains(metadata.ClientHello.SupportedProtos, "http/1.1")
	acceptH2 := e.http2Enabled && common.Contains(metadata.ClientHello.SupportedProtos, "h2")
	if !acceptHTTP && !acceptH2 {
		metadata.MITM = nil
		e.logger.DebugContext(ctx, "unsupported application protocol: ", strings.Join(metadata.ClientHello.SupportedProtos, ","))
		e.connection.NewConnection(ctx, this, conn, metadata, onClose)
		return nil
	}
	var nextProtos []string
	if acceptH2 {
		nextProtos = append(nextProtos, "h2")
	} else if acceptHTTP {
		nextProtos = append(nextProtos, "http/1.1")
	}
	var (
		maxVersion uint16
		minVersion uint16
	)
	for _, version := range metadata.ClientHello.SupportedVersions {
		maxVersion = common.Max(maxVersion, version)
		minVersion = common.Min(minVersion, version)
	}
	serverName := metadata.ClientHello.ServerName
	if serverName == "" && metadata.Destination.IsIP() {
		serverName = metadata.Destination.Addr.String()
	}
	tlsConfig := &tls.Config{
		Time:       e.timeFunc,
		ServerName: serverName,
		NextProtos: nextProtos,
		MinVersion: minVersion,
		MaxVersion: maxVersion,
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return sTLS.GenerateKeyPair(e.tlsCertificate, e.tlsPrivateKey, e.timeFunc, serverName)
		},
	}
	tlsConn := tls.Server(conn, tlsConfig)
	err := tlsConn.HandshakeContext(ctx)
	if err != nil {
		return E.Cause(err, "TLS handshake")
	}
	if tlsConn.ConnectionState().NegotiatedProtocol == "h2" {
		return e.newHTTP2(ctx, this, tlsConn, tlsConfig, metadata, onClose)
	} else {
		return e.newHTTP1(ctx, this, tlsConn, tlsConfig, metadata)
	}
}

func (e *Engine) newHTTP1(ctx context.Context, this N.Dialer, conn net.Conn, tlsConfig *tls.Config, metadata adapter.InboundContext) error {
	options := metadata.MITM
	defer conn.Close()
	reader := bufio.NewReader(conn)
	request, err := sHTTP.ReadRequest(reader)
	if err != nil {
		return E.Cause(err, "read HTTP request")
	}
	rawRequestURL := request.URL
	rawRequestURL.Scheme = "https"
	if rawRequestURL.Host == "" {
		rawRequestURL.Host = request.Host
	}
	requestURL := rawRequestURL.String()
	request.RequestURI = ""
	var (
		requestMatch  bool
		requestScript adapter.HTTPRequestScript
	)
	for _, script := range e.script.Scripts() {
		if !common.Contains(options.Script, script.Tag()) {
			continue
		}
		httpScript, isHTTP := script.(adapter.HTTPRequestScript)
		if !isHTTP {
			_, isHTTP = script.(adapter.HTTPScript)
			if !isHTTP {
				e.logger.WarnContext(ctx, "specified script/", script.Type(), "[", script.Tag(), "] is not a HTTP request/response script")
			}
			continue
		}
		if !httpScript.Match(requestURL) {
			continue
		}
		e.logger.DebugContext(ctx, "match script/", httpScript.Type(), "[", httpScript.Tag(), "]")
		requestScript = httpScript
		requestMatch = true
		break
	}
	var body []byte
	if options.Print && request.ContentLength > 0 && request.ContentLength <= 131072 {
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return E.Cause(err, "read HTTP request body")
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	if options.Print {
		e.printRequest(ctx, request, body)
	}
	if requestScript != nil {
		if body == nil && requestScript.RequiresBody() && request.ContentLength > 0 && (requestScript.MaxSize() == 0 && request.ContentLength <= 131072 || request.ContentLength <= requestScript.MaxSize()) {
			body, err = io.ReadAll(request.Body)
			if err != nil {
				return E.Cause(err, "read HTTP request body")
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
		}
		var result *adapter.HTTPRequestScriptResult
		result, err = requestScript.Run(ctx, request, body)
		if err != nil {
			return E.Cause(err, "execute script/", requestScript.Type(), "[", requestScript.Tag(), "]")
		}
		if result.Response != nil {
			if result.Response.Status == 0 {
				result.Response.Status = http.StatusOK
			}
			response := &http.Response{
				StatusCode: result.Response.Status,
				Status:     http.StatusText(result.Response.Status),
				Proto:      request.Proto,
				ProtoMajor: request.ProtoMajor,
				ProtoMinor: request.ProtoMinor,
				Header:     result.Response.Headers,
				Body:       io.NopCloser(bytes.NewReader(result.Response.Body)),
			}
			err = response.Write(conn)
			if err != nil {
				return E.Cause(err, "write fake response body")
			}
			return nil
		} else {
			if result.URL != "" {
				var newURL *url.URL
				newURL, err = url.Parse(result.URL)
				if err != nil {
					return E.Cause(err, "parse updated request URL")
				}
				request.URL = newURL
				newDestination := M.ParseSocksaddrHostPortStr(newURL.Hostname(), newURL.Port())
				if newDestination.Port == 0 {
					newDestination.Port = metadata.Destination.Port
				}
				metadata.Destination = newDestination
				if tlsConfig != nil {
					tlsConfig.ServerName = newURL.Hostname()
				}
			}
			for key, values := range result.Headers {
				request.Header[key] = values
			}
			if newHost := result.Headers.Get("Host"); newHost != "" {
				request.Host = newHost
				request.Header.Del("Host")
			}
			if result.Body != nil {
				body = result.Body
				request.Body = io.NopCloser(bytes.NewReader(body))
				request.ContentLength = int64(len(body))
			}
		}
	}
	if !requestMatch {
		for i, rule := range options.SurgeURLRewrite {
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			e.logger.DebugContext(ctx, "match url_rewrite[", i, "] => ", rule.String())
			if rule.Reject {
				return E.New("request rejected by url_rewrite")
			} else if rule.Redirect {
				w := new(simpleResponseWriter)
				http.Redirect(w, request, rule.Destination.String(), http.StatusFound)
				err = w.Build(request).Write(conn)
				if err != nil {
					return E.Cause(err, "write url_rewrite 302 response")
				}
				return nil
			}
			requestMatch = true
			request.URL = rule.Destination
			newDestination := M.ParseSocksaddrHostPortStr(rule.Destination.Hostname(), rule.Destination.Port())
			if newDestination.Port == 0 {
				newDestination.Port = metadata.Destination.Port
			}
			metadata.Destination = newDestination
			if tlsConfig != nil {
				tlsConfig.ServerName = rule.Destination.Hostname()
			}
			break
		}
		for i, rule := range options.SurgeHeaderRewrite {
			if rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match header_rewrite[", i, "] => ", rule.String())
			switch {
			case rule.Add:
				if strings.ToLower(rule.Key) == "host" {
					request.Host = rule.Value
					continue
				}
				request.Header.Add(rule.Key, rule.Value)
			case rule.Delete:
				request.Header.Del(rule.Key)
			case rule.Replace:
				if request.Header.Get(rule.Key) != "" {
					request.Header.Set(rule.Key, rule.Value)
				}
			case rule.ReplaceRegex:
				if value := request.Header.Get(rule.Key); value != "" {
					request.Header.Set(rule.Key, rule.Match.ReplaceAllString(value, rule.Value))
				}
			}
		}
		for i, rule := range options.SurgeBodyRewrite {
			if rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match body_rewrite[", i, "] => ", rule.String())
			if body == nil {
				if request.ContentLength <= 0 {
					e.logger.WarnContext(ctx, "body replace skipped due to non-fixed content length")
					break
				} else if request.ContentLength > 131072 {
					e.logger.WarnContext(ctx, "body replace skipped due to large content length: ", request.ContentLength)
					break
				}
				body, err = io.ReadAll(request.Body)
				if err != nil {
					return E.Cause(err, "read HTTP request body")
				}
			}
			for mi := 0; i < len(rule.Match); i++ {
				body = rule.Match[mi].ReplaceAll(body, []byte(rule.Replace[i]))
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
		}
	}
	if !requestMatch {
		for i, rule := range options.SurgeMapLocal {
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match map_local[", i, "] => ", rule.String())
			var (
				statusCode = http.StatusOK
				headers    = make(http.Header)
			)
			if rule.StatusCode > 0 {
				statusCode = rule.StatusCode
			}
			switch {
			case rule.File:
				resource, err := os.ReadFile(rule.Data)
				if err != nil {
					return E.Cause(err, "open map local source")
				}
				mimeType := mime.TypeByExtension(filepath.Ext(rule.Data))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				headers.Set("Content-Type", mimeType)
				body = resource
			case rule.Text:
				headers.Set("Content-Type", "text/plain")
				body = []byte(rule.Data)
			case rule.TinyGif:
				headers.Set("Content-Type", "image/gif")
				body = surgeTinyGif()
			case rule.Base64:
				headers.Set("Content-Type", "application/octet-stream")
				body = rule.Base64Data
			}
			response := &http.Response{
				StatusCode: statusCode,
				Status:     http.StatusText(statusCode),
				Proto:      request.Proto,
				ProtoMajor: request.ProtoMajor,
				ProtoMinor: request.ProtoMinor,
				Header:     headers,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}
			err = response.Write(conn)
			if err != nil {
				return E.Cause(err, "write map local response")
			}
			return nil
		}
	}
	ctx = adapter.WithContext(ctx, &metadata)
	var innerErr atomic.TypedValue[error]
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				if len(metadata.DestinationAddresses) > 0 || metadata.Destination.IsIP() {
					return dialer.DialSerialNetwork(ctx, this, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
				} else {
					return this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
				}
			},
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer httpClient.CloseIdleConnections()
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	response, err := httpClient.Do(request.WithContext(requestCtx))
	if err != nil {
		cancel()
		return E.Errors(innerErr.Load(), err)
	}
	var (
		responseScript adapter.HTTPResponseScript
		responseMatch  bool
	)
	for _, script := range e.script.Scripts() {
		if !common.Contains(options.Script, script.Tag()) {
			continue
		}
		httpScript, isHTTP := script.(adapter.HTTPResponseScript)
		if !isHTTP {
			_, isHTTP = script.(adapter.HTTPScript)
			if !isHTTP {
				e.logger.WarnContext(ctx, "specified script/", script.Type(), "[", script.Tag(), "] is not a HTTP request/response script")
			}
			continue
		}
		if !httpScript.Match(requestURL) {
			continue
		}
		e.logger.DebugContext(ctx, "match script/", httpScript.Type(), "[", httpScript.Tag(), "]")
		responseScript = httpScript
		responseMatch = true
		break
	}
	var responseBody []byte
	if options.Print && response.ContentLength > 0 && response.ContentLength <= 131072 {
		responseBody, err = io.ReadAll(response.Body)
		if err != nil {
			return E.Cause(err, "read HTTP response body")
		}
		response.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	if options.Print {
		e.printResponse(ctx, response, responseBody)
	}
	if responseScript != nil {
		if responseBody == nil && responseScript.RequiresBody() && response.ContentLength > 0 && (responseScript.MaxSize() == 0 && response.ContentLength <= 131072 || response.ContentLength <= responseScript.MaxSize()) {
			responseBody, err = io.ReadAll(response.Body)
			if err != nil {
				return E.Cause(err, "read HTTP response body")
			}
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
		var result *adapter.HTTPResponseScriptResult
		result, err = responseScript.Run(ctx, request, response, responseBody)
		if err != nil {
			return E.Cause(err, "execute script/", responseScript.Type(), "[", responseScript.Tag(), "]")
		}
		if result.Status > 0 {
			response.Status = http.StatusText(result.Status)
			response.StatusCode = result.Status
		}
		for key, values := range result.Headers {
			response.Header[key] = values
		}
		if result.Body != nil {
			response.Body.Close()
			responseBody = result.Body
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
			response.ContentLength = int64(len(responseBody))
		}
	}
	if !responseMatch {
		for i, rule := range options.SurgeHeaderRewrite {
			if !rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			responseMatch = true
			e.logger.DebugContext(ctx, "match header_rewrite[", i, "] => ", rule.String())
			switch {
			case rule.Add:
				response.Header.Add(rule.Key, rule.Value)
			case rule.Delete:
				response.Header.Del(rule.Key)
			case rule.Replace:
				if response.Header.Get(rule.Key) != "" {
					response.Header.Set(rule.Key, rule.Value)
				}
			case rule.ReplaceRegex:
				if value := response.Header.Get(rule.Key); value != "" {
					response.Header.Set(rule.Key, rule.Match.ReplaceAllString(value, rule.Value))
				}
			}
		}
		for i, rule := range options.SurgeBodyRewrite {
			if !rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			responseMatch = true
			e.logger.DebugContext(ctx, "match body_rewrite[", i, "] => ", rule.String())
			if responseBody == nil {
				if response.ContentLength <= 0 {
					e.logger.WarnContext(ctx, "body replace skipped due to non-fixed content length")
					break
				} else if response.ContentLength > 131072 {
					e.logger.WarnContext(ctx, "body replace skipped due to large content length: ", request.ContentLength)
					break
				}
				responseBody, err = io.ReadAll(response.Body)
				if err != nil {
					return E.Cause(err, "read HTTP request body")
				}
			}
			for mi := 0; i < len(rule.Match); i++ {
				responseBody = rule.Match[mi].ReplaceAll(responseBody, []byte(rule.Replace[i]))
			}
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
			response.ContentLength = int64(len(responseBody))
		}
	}
	if !options.Print && !requestMatch && !responseMatch {
		e.logger.WarnContext(ctx, "request not modified")
	}
	err = response.Write(conn)
	if err != nil {
		return E.Errors(E.Cause(err, "write HTTP response"), innerErr.Load())
	} else if innerErr.Load() != nil {
		return E.Cause(innerErr.Load(), "write HTTP response")
	}
	return nil
}

func (e *Engine) newHTTP2(ctx context.Context, this N.Dialer, conn net.Conn, tlsConfig *tls.Config, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) error {
	handler := &engineHandler{
		Engine:    e,
		conn:      conn,
		tlsConfig: tlsConfig,
		dialer:    this,
		metadata:  metadata,
		httpClient: &http.Client{
			Transport: &http2.Transport{
				AllowHTTP:        true,
				MaxReadFrameSize: math.MaxUint32,
				DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
					ctx = adapter.WithContext(ctx, &metadata)
					var (
						remoteConn net.Conn
						err        error
					)
					if len(metadata.DestinationAddresses) > 0 || metadata.Destination.IsIP() {
						remoteConn, err = dialer.DialSerialNetwork(ctx, this, N.NetworkTCP, metadata.Destination, metadata.DestinationAddresses, metadata.NetworkStrategy, metadata.NetworkType, metadata.FallbackNetworkType, metadata.FallbackDelay)
					} else {
						remoteConn, err = this.DialContext(ctx, N.NetworkTCP, metadata.Destination)
					}
					if err != nil {
						return nil, err
					}
					return tls.Client(remoteConn, cfg), nil
				},
				TLSClientConfig: tlsConfig,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		onClose: onClose,
	}
	http2Server := &http2.Server{
		MaxReadFrameSize: math.MaxUint32,
	}
	http2Server.ServeConn(conn, &http2.ServeConnOpts{
		Context: ctx,
		Handler: handler,
	})
	return nil
}

type engineHandler struct {
	*Engine
	conn       net.Conn
	tlsConfig  *tls.Config
	dialer     N.Dialer
	metadata   adapter.InboundContext
	onClose    N.CloseHandlerFunc
	httpClient *http.Client
}

func (e *engineHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	err := e.serveHTTP(request.Context(), writer, request)
	if err != nil {
		e.conn.Close()
		if E.IsClosedOrCanceled(err) {
			e.logger.DebugContext(request.Context(), E.Cause(err, "connection closed"))
		} else {
			e.logger.ErrorContext(request.Context(), err)
		}
	}
}

func (e *engineHandler) serveHTTP(ctx context.Context, writer http.ResponseWriter, request *http.Request) error {
	options := e.metadata.MITM
	rawRequestURL := request.URL
	rawRequestURL.Scheme = "https"
	if rawRequestURL.Host == "" {
		rawRequestURL.Host = request.Host
	}
	requestURL := rawRequestURL.String()
	request.RequestURI = ""
	var (
		requestMatch  bool
		requestScript adapter.HTTPRequestScript
	)
	for _, script := range e.script.Scripts() {
		if !common.Contains(options.Script, script.Tag()) {
			continue
		}
		httpScript, isHTTP := script.(adapter.HTTPRequestScript)
		if !isHTTP {
			_, isHTTP = script.(adapter.HTTPScript)
			if !isHTTP {
				e.logger.WarnContext(ctx, "specified script/", script.Type(), "[", script.Tag(), "] is not a HTTP request/response script")
			}
			continue
		}
		if !httpScript.Match(requestURL) {
			continue
		}
		e.logger.DebugContext(ctx, "match script/", httpScript.Type(), "[", httpScript.Tag(), "]")
		requestScript = httpScript
		requestMatch = true
		break
	}
	var (
		body []byte
		err  error
	)
	if options.Print && request.ContentLength > 0 && request.ContentLength <= 131072 {
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return E.Cause(err, "read HTTP request body")
		}
		request.Body.Close()
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
	if options.Print {
		e.printRequest(ctx, request, body)
	}
	if requestScript != nil {
		if body == nil && requestScript.RequiresBody() && request.ContentLength > 0 && (requestScript.MaxSize() == 0 && request.ContentLength <= 131072 || request.ContentLength <= requestScript.MaxSize()) {
			body, err = io.ReadAll(request.Body)
			if err != nil {
				return E.Cause(err, "read HTTP request body")
			}
			request.Body.Close()
			request.Body = io.NopCloser(bytes.NewReader(body))
		}
		result, err := requestScript.Run(ctx, request, body)
		if err != nil {
			return E.Cause(err, "execute script/", requestScript.Type(), "[", requestScript.Tag(), "]")
		}
		if result.Response != nil {
			if result.Response.Status == 0 {
				result.Response.Status = http.StatusOK
			}
			for key, values := range result.Response.Headers {
				writer.Header()[key] = values
			}
			writer.WriteHeader(result.Response.Status)
			if result.Response.Body != nil {
				_, err = writer.Write(result.Response.Body)
				if err != nil {
					return E.Cause(err, "write fake response body")
				}
			}
			return nil
		} else {
			if result.URL != "" {
				var newURL *url.URL
				newURL, err = url.Parse(result.URL)
				if err != nil {
					return E.Cause(err, "parse updated request URL")
				}
				request.URL = newURL
				newDestination := M.ParseSocksaddrHostPortStr(newURL.Hostname(), newURL.Port())
				if newDestination.Port == 0 {
					newDestination.Port = e.metadata.Destination.Port
				}
				e.metadata.Destination = newDestination
				e.tlsConfig.ServerName = newURL.Hostname()
			}
			for key, values := range result.Headers {
				request.Header[key] = values
			}
			if newHost := result.Headers.Get("Host"); newHost != "" {
				request.Host = newHost
				request.Header.Del("Host")
			}
			if result.Body != nil {
				io.Copy(io.Discard, request.Body)
				request.Body = io.NopCloser(bytes.NewReader(result.Body))
				request.ContentLength = int64(len(result.Body))
			}
		}
	}
	if !requestMatch {
		for i, rule := range options.SurgeURLRewrite {
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			e.logger.DebugContext(ctx, "match url_rewrite[", i, "] => ", rule.String())
			if rule.Reject {
				return E.New("request rejected by url_rewrite")
			} else if rule.Redirect {
				http.Redirect(writer, request, rule.Destination.String(), http.StatusFound)
				return nil
			}
			requestMatch = true
			request.URL = rule.Destination
			newDestination := M.ParseSocksaddrHostPortStr(rule.Destination.Hostname(), rule.Destination.Port())
			if newDestination.Port == 0 {
				newDestination.Port = e.metadata.Destination.Port
			}
			e.metadata.Destination = newDestination
			e.tlsConfig.ServerName = rule.Destination.Hostname()
			break
		}
		for i, rule := range options.SurgeHeaderRewrite {
			if rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match header_rewrite[", i, "] => ", rule.String())
			switch {
			case rule.Add:
				if strings.ToLower(rule.Key) == "host" {
					request.Host = rule.Value
					continue
				}
				request.Header.Add(rule.Key, rule.Value)
			case rule.Delete:
				request.Header.Del(rule.Key)
			case rule.Replace:
				if request.Header.Get(rule.Key) != "" {
					request.Header.Set(rule.Key, rule.Value)
				}
			case rule.ReplaceRegex:
				if value := request.Header.Get(rule.Key); value != "" {
					request.Header.Set(rule.Key, rule.Match.ReplaceAllString(value, rule.Value))
				}
			}
		}
		for i, rule := range options.SurgeBodyRewrite {
			if rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match body_rewrite[", i, "] => ", rule.String())
			var body []byte
			if request.ContentLength <= 0 {
				e.logger.WarnContext(ctx, "body replace skipped due to non-fixed content length")
				break
			} else if request.ContentLength > 131072 {
				e.logger.WarnContext(ctx, "body replace skipped due to large content length: ", request.ContentLength)
				break
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return E.Cause(err, "read HTTP request body")
			}
			request.Body.Close()
			for mi := 0; i < len(rule.Match); i++ {
				body = rule.Match[mi].ReplaceAll(body, []byte(rule.Replace[i]))
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
		}
	}
	if !requestMatch {
		for i, rule := range options.SurgeMapLocal {
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			requestMatch = true
			e.logger.DebugContext(ctx, "match map_local[", i, "] => ", rule.String())
			go func() {
				io.Copy(io.Discard, request.Body)
				request.Body.Close()
			}()
			var (
				statusCode = http.StatusOK
				headers    = make(http.Header)
				body       []byte
			)
			if rule.StatusCode > 0 {
				statusCode = rule.StatusCode
			}
			switch {
			case rule.File:
				resource, err := os.ReadFile(rule.Data)
				if err != nil {
					return E.Cause(err, "open map local source")
				}
				mimeType := mime.TypeByExtension(filepath.Ext(rule.Data))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				headers.Set("Content-Type", mimeType)
				body = resource
			case rule.Text:
				headers.Set("Content-Type", "text/plain")
				body = []byte(rule.Data)
			case rule.TinyGif:
				headers.Set("Content-Type", "image/gif")
				body = surgeTinyGif()
			case rule.Base64:
				headers.Set("Content-Type", "application/octet-stream")
				body = rule.Base64Data
			}
			for key, values := range headers {
				writer.Header()[key] = values
			}
			writer.WriteHeader(statusCode)
			_, err = writer.Write(body)
			if err != nil {
				return E.Cause(err, "write map local response")
			}
			return nil
		}
	}
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	response, err := e.httpClient.Do(request.WithContext(requestCtx))
	if err != nil {
		cancel()
		return E.Cause(err, "exchange request")
	}
	var (
		responseScript adapter.HTTPResponseScript
		responseMatch  bool
	)
	for _, script := range e.script.Scripts() {
		if !common.Contains(options.Script, script.Tag()) {
			continue
		}
		httpScript, isHTTP := script.(adapter.HTTPResponseScript)
		if !isHTTP {
			_, isHTTP = script.(adapter.HTTPScript)
			if !isHTTP {
				e.logger.WarnContext(ctx, "specified script/", script.Type(), "[", script.Tag(), "] is not a HTTP request/response script")
			}
			continue
		}
		if !httpScript.Match(requestURL) {
			continue
		}
		e.logger.DebugContext(ctx, "match script/", httpScript.Type(), "[", httpScript.Tag(), "]")
		responseScript = httpScript
		responseMatch = true
		break
	}
	var responseBody []byte
	if options.Print && response.ContentLength > 0 && response.ContentLength <= 131072 {
		responseBody, err = io.ReadAll(response.Body)
		if err != nil {
			return E.Cause(err, "read HTTP response body")
		}
		response.Body.Close()
		response.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	if options.Print {
		e.printResponse(ctx, response, responseBody)
	}
	if responseScript != nil {
		if responseBody == nil && responseScript.RequiresBody() && response.ContentLength > 0 && (responseScript.MaxSize() == 0 && response.ContentLength <= 131072 || response.ContentLength <= responseScript.MaxSize()) {
			responseBody, err = io.ReadAll(response.Body)
			if err != nil {
				return E.Cause(err, "read HTTP response body")
			}
			response.Body.Close()
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
		}
		var result *adapter.HTTPResponseScriptResult
		result, err = responseScript.Run(ctx, request, response, responseBody)
		if err != nil {
			return E.Cause(err, "execute script/", responseScript.Type(), "[", responseScript.Tag(), "]")
		}
		if result.Status > 0 {
			response.Status = http.StatusText(result.Status)
			response.StatusCode = result.Status
		}
		for key, values := range result.Headers {
			response.Header[key] = values
		}
		if result.Body != nil {
			response.Body.Close()
			response.Body = io.NopCloser(bytes.NewReader(result.Body))
			response.ContentLength = int64(len(result.Body))
		}
	}
	if !responseMatch {
		for i, rule := range options.SurgeHeaderRewrite {
			if !rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			responseMatch = true
			e.logger.DebugContext(ctx, "match header_rewrite[", i, "] => ", rule.String())
			switch {
			case rule.Add:
				response.Header.Add(rule.Key, rule.Value)
			case rule.Delete:
				response.Header.Del(rule.Key)
			case rule.Replace:
				if response.Header.Get(rule.Key) != "" {
					response.Header.Set(rule.Key, rule.Value)
				}
			case rule.ReplaceRegex:
				if value := response.Header.Get(rule.Key); value != "" {
					response.Header.Set(rule.Key, rule.Match.ReplaceAllString(value, rule.Value))
				}
			}
		}
		for i, rule := range options.SurgeBodyRewrite {
			if !rule.Response {
				continue
			}
			if !rule.Pattern.MatchString(requestURL) {
				continue
			}
			responseMatch = true
			e.logger.DebugContext(ctx, "match body_rewrite[", i, "] => ", rule.String())
			if responseBody == nil {
				if response.ContentLength <= 0 {
					e.logger.WarnContext(ctx, "body replace skipped due to non-fixed content length")
					break
				} else if response.ContentLength > 131072 {
					e.logger.WarnContext(ctx, "body replace skipped due to large content length: ", request.ContentLength)
					break
				}
				responseBody, err = io.ReadAll(response.Body)
				if err != nil {
					return E.Cause(err, "read HTTP request body")
				}
				response.Body.Close()
			}
			for mi := 0; i < len(rule.Match); i++ {
				responseBody = rule.Match[mi].ReplaceAll(responseBody, []byte(rule.Replace[i]))
			}
			response.Body = io.NopCloser(bytes.NewReader(responseBody))
			response.ContentLength = int64(len(responseBody))
		}
	}
	if !options.Print && !requestMatch && !responseMatch {
		e.logger.WarnContext(ctx, "request not modified")
	}
	for key, values := range response.Header {
		writer.Header()[key] = values
	}
	writer.WriteHeader(response.StatusCode)
	_, err = io.Copy(writer, response.Body)
	response.Body.Close()
	if err != nil {
		return E.Cause(err, "write HTTP response")
	}
	return nil
}

func (e *Engine) printRequest(ctx context.Context, request *http.Request, body []byte) {
	e.logger.TraceContext(ctx, "request: ", request.Proto, " ", request.Method, " ", request.URL.String())
	if request.URL.Hostname() != "" && request.URL.Hostname() != request.Host {
		e.logger.TraceContext(ctx, "request: ", "Host: ", request.Host)
	}
	for key, values := range request.Header {
		for _, value := range values {
			e.logger.TraceContext(ctx, "request: ", key, ": ", value)
		}
	}
	if len(body) > 0 {
		if !bytes.ContainsFunc(body, func(r rune) bool {
			return !unicode.IsPrint(r) && !unicode.IsSpace(r)
		}) {
			e.logger.TraceContext(ctx, "request: body: ", string(body))
		} else {
			e.logger.TraceContext(ctx, "request: body unprintable")
		}
	}
}

func (e *Engine) printResponse(ctx context.Context, response *http.Response, body []byte) {
	e.logger.TraceContext(ctx, "response: ", response.Proto, " ", response.Status)
	for key, values := range response.Header {
		for _, value := range values {
			e.logger.TraceContext(ctx, "response: ", key, ": ", value)
		}
	}
	if len(body) > 0 {
		if !bytes.ContainsFunc(body, func(r rune) bool {
			return !unicode.IsPrint(r) && !unicode.IsSpace(r)
		}) {
			e.logger.TraceContext(ctx, "response: ", string(body))
		}
	} else {
		e.logger.TraceContext(ctx, "response: body unprintable")
	}
}

type simpleResponseWriter struct {
	statusCode int
	header     http.Header
	body       bytes.Buffer
}

func (w *simpleResponseWriter) Build(request *http.Request) *http.Response {
	return &http.Response{
		StatusCode: w.statusCode,
		Status:     http.StatusText(w.statusCode),
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Header:     w.header,
		Body:       io.NopCloser(&w.body),
	}
}

func (w *simpleResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *simpleResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *simpleResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
