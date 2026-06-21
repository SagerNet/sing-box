package libbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	"github.com/sagernet/sing-box/common/certificate"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

type HTTPClient interface {
	RestrictedTLS()
	ModernTLS()
	PinnedTLS12()
	PinnedSHA256(sumHex string)
	TrySocks5(port int32)
	KeepAlive()
	NewRequest() HTTPRequest
	Close()
}

type HTTPRequest interface {
	SetURL(link string) error
	SetMethod(method string)
	SetHeader(key string, value string)
	SetContent(content []byte)
	SetContentString(content string)
	RandomUserAgent()
	SetUserAgent(userAgent string)
	Execute() (HTTPResponse, error)
}

type HTTPResponse interface {
	GetContent() (*StringBox, error)
	WriteTo(path string) error
	WriteToWithProgress(path string, handler HTTPResponseWriteToProgressHandler) error
}

type HTTPResponseWriteToProgressHandler interface {
	Update(progress int64, total int64)
}

var (
	_ HTTPClient   = (*httpClient)(nil)
	_ HTTPRequest  = (*httpRequest)(nil)
	_ HTTPResponse = (*httpResponse)(nil)
)

type httpClient struct {
	tls       tls.Config
	client    http.Client
	transport http.Transport
	store     *certificate.Store
}

func NewHTTPClient() HTTPClient {
	client := new(httpClient)
	client.client.Transport = &client.transport
	client.transport.ForceAttemptHTTP2 = true
	client.transport.TLSHandshakeTimeout = C.TCPTimeout
	client.transport.TLSClientConfig = &client.tls
	client.transport.DisableKeepAlives = true
	if C.IsAndroid {
		store, err := certificate.NewStore(logger.NOP(), option.CertificateOptions{})
		if err != nil {
			panic(E.Cause(err, "initialize certificate store"))
		}
		client.tls.RootCAs = store.Pool()
	}
	return client
}

func (c *httpClient) ModernTLS() {
	c.setTLSVersion(tls.VersionTLS12, 0, func(suite *tls.CipherSuite) bool { return true })
}

func (c *httpClient) RestrictedTLS() {
	c.setTLSVersion(tls.VersionTLS13, 0, func(suite *tls.CipherSuite) bool {
		return common.Contains(suite.SupportedVersions, uint16(tls.VersionTLS13))
	})
}

func (c *httpClient) setTLSVersion(minVersion, maxVersion uint16, filter func(*tls.CipherSuite) bool) {
	c.tls.MinVersion = minVersion
	if maxVersion != 0 {
		c.tls.MaxVersion = maxVersion
	}
	c.tls.CipherSuites = common.Map(common.Filter(tls.CipherSuites(), filter), func(it *tls.CipherSuite) uint16 {
		return it.ID
	})
}

func (c *httpClient) PinnedTLS12() {
	c.setTLSVersion(tls.VersionTLS12, tls.VersionTLS12, func(suite *tls.CipherSuite) bool { return true })
}

func (c *httpClient) PinnedSHA256(sumHex string) {
	c.tls.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		for _, rawCert := range rawCerts {
			certSum := sha256.Sum256(rawCert)
			if sumHex == hex.EncodeToString(certSum[:]) {
				return nil
			}
		}
		return E.New("pinned sha256 sum mismatch")
	}
}

func (c *httpClient) TrySocks5(port int32) {
	dialer := new(net.Dialer)
	c.transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		for {
			socksConn, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:"+strconv.Itoa(int(port)))
			if err != nil {
				break
			}
			_, err = socks.ClientHandshake5(socksConn, socks5.CommandConnect, M.ParseSocksaddr(addr), "", "")
			if err != nil {
				break
			}
			//nolint:staticcheck
			return socksConn, err
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

func (c *httpClient) KeepAlive() {
	c.transport.DisableKeepAlives = false
}

func (c *httpClient) NewRequest() HTTPRequest {
	req := &httpRequest{httpClient: c}
	req.request = http.Request{
		Method: "GET",
		Header: http.Header{},
	}
	return req
}

func (c *httpClient) Close() {
	if c.store != nil {
		c.store.Close()
	}
	c.transport.CloseIdleConnections()
}

type httpRequest struct {
	*httpClient
	request http.Request
}

func (r *httpRequest) SetURL(link string) (err error) {
	r.request.URL, err = url.Parse(link)
	if err != nil {
		return
	}
	if r.request.URL.User != nil {
		user := r.request.URL.User.Username()
		password, _ := r.request.URL.User.Password()
		r.request.SetBasicAuth(user, password)
	}
	return
}

func (r *httpRequest) SetMethod(method string) {
	r.request.Method = method
}

func (r *httpRequest) SetHeader(key string, value string) {
	r.request.Header.Set(key, value)
}

func (r *httpRequest) RandomUserAgent() {
	r.request.Header.Set("User-Agent", fmt.Sprintf("curl/7.%d.%d", rand.Int()%54, rand.Int()%2))
}

func (r *httpRequest) SetUserAgent(userAgent string) {
	r.request.Header.Set("User-Agent", userAgent)
}

func (r *httpRequest) SetContent(content []byte) {
	r.request.Body = io.NopCloser(bytes.NewReader(content))
	r.request.ContentLength = int64(len(content))
}

func (r *httpRequest) SetContentString(content string) {
	r.SetContent([]byte(content))
}

func (r *httpRequest) Execute() (HTTPResponse, error) {
	response, err := r.client.Do(&r.request)
	if err != nil {
		return nil, err
	}
	httpResp := &httpResponse{Response: response}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New(httpResp.errorString())
	}
	return httpResp, nil
}

type httpResponse struct {
	*http.Response

	getContentOnce sync.Once
	content        []byte
	contentError   error
}

func (h *httpResponse) errorString() string {
	content, err := h.GetContent()
	if err != nil {
		return fmt.Sprint("HTTP ", h.Status)
	}
	return fmt.Sprint("HTTP ", h.Status, ": ", content)
}

func (h *httpResponse) GetContent() (*StringBox, error) {
	h.getContentOnce.Do(func() {
		defer h.Body.Close()
		h.content, h.contentError = io.ReadAll(h.Body)
	})
	if h.contentError != nil {
		return nil, h.contentError
	}
	return wrapString(string(h.content)), nil
}

func (h *httpResponse) WriteTo(path string) error {
	defer h.Body.Close()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return common.Error(bufio.Copy(file, h.Body))
}

func (h *httpResponse) WriteToWithProgress(path string, handler HTTPResponseWriteToProgressHandler) error {
	defer h.Body.Close()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return common.Error(bufio.Copy(&progressWriter{
		writer:  file,
		handler: handler,
		total:   h.ContentLength,
	}, h.Body))
}

type progressWriter struct {
	writer  io.Writer
	handler HTTPResponseWriteToProgressHandler
	total   int64
	written int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.written += int64(n)
	w.handler.Update(w.written, w.total)
	return n, err
}
