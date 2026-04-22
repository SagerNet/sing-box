//go:build windows

package winhttp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

var versionCheck = sync.OnceValue(func() error {
	major, _, build := windows.RtlGetNtVersionNumbers()
	build &= 0xffff
	if major < 10 || (major == 10 && build < 19041) {
		return E.New("Windows HTTP engine requires Windows build 19041 or later (Windows 10 version 2004, Windows Server 2022, or newer)")
	}
	return nil
})

func CheckPlatform() error {
	return versionCheck()
}

type Session struct {
	handle atomic.Uintptr
}

type Connection struct {
	handle atomic.Uintptr
}

type Request struct {
	handle atomic.Uintptr
}

func OpenSession(userAgent string) (*Session, error) {
	err := CheckPlatform()
	if err != nil {
		return nil, err
	}
	agent16, err := windows.UTF16PtrFromString(userAgent)
	if err != nil {
		return nil, err
	}
	handle, err := winHTTPOpen(agent16, accessTypeNoProxy, nil, nil, FlagAsync)
	if err != nil {
		return nil, wrapError("open session", err)
	}
	session := &Session{}
	session.handle.Store(uintptr(handle))
	return session, nil
}

func (s *Session) Close() error {
	handle := s.take()
	if handle == 0 {
		return nil
	}
	err := winHTTPCloseHandle(handle)
	return wrapError("close session", err)
}

func (s *Session) SetStatusCallback(callback uintptr, flags uint32) error {
	_, err := winHTTPSetStatusCallback(s.load(), callback, flags, 0)
	return wrapError("set session status callback", err)
}

func (s *Session) SetTimeouts(resolveTimeout, connectTimeout, sendTimeout, receiveTimeout int32) error {
	err := winHTTPSetTimeouts(s.load(), resolveTimeout, connectTimeout, sendTimeout, receiveTimeout)
	return wrapError("set session timeouts", err)
}

func (s *Session) SetProxy(proxy string) error {
	proxy16, err := windows.UTF16PtrFromString(proxy)
	if err != nil {
		return err
	}
	info := ProxyInfo{
		AccessType: accessTypeNamedProxy,
		Proxy:      proxy16,
	}
	err = s.setOption(optionProxy, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)))
	return wrapError("set session proxy", err)
}

func (s *Session) DisableGlobalPooling() error {
	err := s.setUint32Option(optionDisableGlobalPooling, 1)
	return wrapError("disable session global pooling", err)
}

func (s *Session) SetSecureProtocols(flags uint32) error {
	err := s.setUint32Option(optionSecureProtocols, flags)
	return wrapError("set session secure protocols", err)
}

func (s *Session) Connect(serverName string, port uint16) (*Connection, error) {
	serverName16, err := windows.UTF16PtrFromString(serverName)
	if err != nil {
		return nil, err
	}
	handle, err := winHTTPConnect(s.load(), serverName16, port, 0)
	if err != nil {
		return nil, wrapError("connect session", err)
	}
	connection := &Connection{}
	connection.handle.Store(uintptr(handle))
	return connection, nil
}

func (c *Connection) Close() error {
	handle := c.take()
	if handle == 0 {
		return nil
	}
	err := winHTTPCloseHandle(handle)
	return wrapError("close connection", err)
}

func (c *Connection) OpenRequest(method, objectName string, secure bool) (*Request, error) {
	method16, err := windows.UTF16PtrFromString(method)
	if err != nil {
		return nil, err
	}
	object16, err := windows.UTF16PtrFromString(objectName)
	if err != nil {
		return nil, err
	}
	var flags uint32
	if secure {
		flags |= FlagSecure
	}
	handle, err := winHTTPOpenRequest(c.load(), method16, object16, nil, nil, nil, flags)
	if err != nil {
		return nil, wrapError("open request", err)
	}
	request := &Request{}
	request.handle.Store(uintptr(handle))
	return request, nil
}

func (r *Request) Close() error {
	handle := r.take()
	if handle == 0 {
		return nil
	}
	err := winHTTPCloseHandle(handle)
	return wrapError("close request", err)
}

func (r *Request) SetContextValue(value uintptr) error {
	err := r.setOption(optionContextValue, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)))
	return wrapError("set request context value", err)
}

func (r *Request) SetRedirectPolicy(policy uint32) error {
	err := r.setUint32Option(optionRedirectPolicy, policy)
	return wrapError("set request redirect policy", err)
}

func (r *Request) SetDisableFeature(flags uint32) error {
	err := r.setUint32Option(optionDisableFeature, flags)
	return wrapError("set request disable feature", err)
}

func (r *Request) SetSecureProtocols(flags uint32) error {
	err := r.setUint32Option(optionSecureProtocols, flags)
	return wrapError("set request secure protocols", err)
}

func (r *Request) SetSecurityFlags(flags uint32) error {
	err := r.setUint32Option(optionSecurityFlags, flags)
	return wrapError("set request security flags", err)
}

func (r *Request) SetEnableHTTPProtocol(flags uint32) error {
	err := r.setUint32Option(optionEnableHTTPProtocol, flags)
	return wrapError("set request enabled HTTP protocol", err)
}

func (r *Request) SetRequiredHTTPProtocol(flags uint32) error {
	err := r.setUint32Option(optionHTTPProtocolRequired, flags)
	return wrapError("set request required HTTP protocol", err)
}

func (r *Request) SetProxyCredentials(username, password string) error {
	if username != "" {
		err := r.setStringOption(optionProxyUsername, username)
		if err != nil {
			return wrapError("set request proxy username", err)
		}
	}
	if password != "" {
		err := r.setStringOption(optionProxyPassword, password)
		if err != nil {
			return wrapError("set request proxy password", err)
		}
	}
	return nil
}

func (r *Request) DisableProxyAuthSchemes(flags uint32) error {
	err := r.setUint32Option(optionDisableProxyAuthSchemes, flags)
	return wrapError("disable request proxy auth schemes", err)
}

func (r *Request) Send(headers []uint16, body []byte) error {
	var (
		headers16 *uint16
		headerLen uint32
		bodyPtr   *byte
	)
	if len(headers) > 0 {
		headers16 = &headers[0]
		headerLen = ^uint32(0)
	}
	if len(body) > 0 {
		bodyPtr = &body[0]
	}
	err := winHTTPSendRequest(r.load(), headers16, headerLen, bodyPtr, uint32(len(body)), uint32(len(body)), 0)
	return startAsync("send request", err)
}

func (r *Request) ReceiveResponse() error {
	err := winHTTPReceiveResponse(r.load(), 0)
	return startAsync("receive response", err)
}

func (r *Request) ReadData(buffer []byte) error {
	if len(buffer) == 0 {
		return os.ErrInvalid
	}
	err := winHTTPReadData(r.load(), &buffer[0], uint32(len(buffer)), nil)
	return startAsync("read response body", err)
}

func (r *Request) QueryStatusCode() (int, error) {
	var value uint32
	size := uint32(unsafe.Sizeof(value))
	err := winHTTPQueryHeaders(r.load(), queryStatusCode|queryFlagNumber, nil, unsafe.Pointer(&value), &size, nil)
	if err != nil {
		return 0, wrapError("query response status code", err)
	}
	return int(value), nil
}

func (r *Request) QueryVersion() (string, error) {
	version, err := r.queryHeaderString(queryVersion)
	if err != nil {
		return "", err
	}
	return version, nil
}

func (r *Request) QueryRawHeadersCRLF() (string, error) {
	value, err := r.queryHeaderString(queryRawHeadersCRLF)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *Request) QueryHTTPProtocolUsed() (uint32, error) {
	return r.queryUint32Option(optionHTTPProtocolUsed, "query request HTTP protocol")
}

func (r *Request) QuerySecurityInfo() (SecurityInfo, error) {
	var value SecurityInfo
	size := uint32(unsafe.Sizeof(value))
	err := winHTTPQueryOption(r.load(), optionSecurityInfo, unsafe.Pointer(&value), &size)
	if err != nil {
		return SecurityInfo{}, wrapError("query request security info", err)
	}
	return value, nil
}

func (r *Request) QueryServerCertContext() (*windows.CertContext, error) {
	var value *windows.CertContext
	size := uint32(unsafe.Sizeof(value))
	err := winHTTPQueryOption(r.load(), optionServerCertContext, unsafe.Pointer(&value), &size)
	if err != nil {
		return nil, wrapError("query request server certificate context", err)
	}
	return value, nil
}

func (r *Request) QueryServerCertChainContext() (*windows.CertChainContext, error) {
	var value *windows.CertChainContext
	size := uint32(unsafe.Sizeof(value))
	err := winHTTPQueryOption(r.load(), optionServerCertChainContext, unsafe.Pointer(&value), &size)
	if err != nil {
		return nil, wrapError("query request server certificate chain context", err)
	}
	return value, nil
}

func (r *Request) QueryServerCertificateChainDER() ([][]byte, error) {
	chainCtx, err := r.QueryServerCertChainContext()
	if err != nil {
		if errors.Is(err, ErrorInvalidOption) || errors.Is(err, ErrorIncorrectHandleState) {
			der, certErr := r.QueryServerCertificateDER()
			if certErr == nil {
				return der, nil
			}
		}
		return nil, err
	}
	if chainCtx == nil {
		return nil, nil
	}
	defer windows.CertFreeCertificateChain(chainCtx)
	chain, err := extractCertificateChainDER(chainCtx)
	if err == nil {
		return chain, nil
	}
	return r.QueryServerCertificateDER()
}

func (r *Request) QueryServerCertificateDER() ([][]byte, error) {
	certCtx, err := r.QueryServerCertContext()
	if err != nil {
		if errors.Is(err, ErrorInvalidOption) {
			return nil, nil
		}
		return nil, err
	}
	if certCtx == nil {
		return nil, nil
	}
	defer windows.CertFreeCertificateContext(certCtx)
	certDER := certificateContextDER(certCtx)
	if len(certDER) == 0 {
		return nil, nil
	}
	return [][]byte{certDER}, nil
}

func (s *Session) setStringOption(option uint32, value string) error {
	value16, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	return s.setOption(option, unsafe.Pointer(value16), uint32((len(value)+1)*2))
}

func (s *Session) setUint32Option(option uint32, value uint32) error {
	return s.setOption(option, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)))
}

func (s *Session) setOption(option uint32, buffer unsafe.Pointer, bufferLen uint32) error {
	return winHTTPSetOption(s.load(), option, buffer, bufferLen)
}

func (r *Request) setUint32Option(option uint32, value uint32) error {
	return r.setOption(option, unsafe.Pointer(&value), uint32(unsafe.Sizeof(value)))
}

func (r *Request) setStringOption(option uint32, value string) error {
	value16, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	return r.setOption(option, unsafe.Pointer(value16), uint32((len(value)+1)*2))
}

func (r *Request) setOption(option uint32, buffer unsafe.Pointer, bufferLen uint32) error {
	return winHTTPSetOption(r.load(), option, buffer, bufferLen)
}

func (r *Request) queryHeaderString(infoLevel uint32) (string, error) {
	size := uint32(0)
	err := winHTTPQueryHeaders(r.load(), infoLevel, nil, nil, &size, nil)
	if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return "", wrapError("query response header", err)
	}
	buffer := make([]uint16, size/2)
	err = winHTTPQueryHeaders(r.load(), infoLevel, nil, unsafe.Pointer(&buffer[0]), &size, nil)
	if err != nil {
		return "", wrapError("query response header", err)
	}
	return windows.UTF16ToString(buffer), nil
}

func (r *Request) queryUint32Option(option uint32, op string) (uint32, error) {
	var value uint32
	size := uint32(unsafe.Sizeof(value))
	err := winHTTPQueryOption(r.load(), option, unsafe.Pointer(&value), &size)
	if err != nil {
		return 0, wrapError(op, err)
	}
	return value, nil
}

func certificateContextDER(ctx *windows.CertContext) []byte {
	if ctx == nil || ctx.EncodedCert == nil || ctx.Length == 0 {
		return nil
	}
	der := make([]byte, ctx.Length)
	copy(der, unsafe.Slice(ctx.EncodedCert, int(ctx.Length)))
	return der
}

func extractCertificateChainDER(chainCtx *windows.CertChainContext) ([][]byte, error) {
	if chainCtx == nil || chainCtx.ChainCount == 0 || chainCtx.Chains == nil {
		return nil, E.New("empty certificate chain")
	}
	chains := unsafe.Slice(chainCtx.Chains, int(chainCtx.ChainCount))
	chain := chains[0]
	if chain == nil || chain.NumElements == 0 || chain.Elements == nil {
		return nil, E.New("empty certificate chain")
	}
	elements := unsafe.Slice(chain.Elements, int(chain.NumElements))
	if len(elements) > 1 &&
		chain.TrustStatus.ErrorStatus&windows.CERT_TRUST_IS_PARTIAL_CHAIN == 0 &&
		isSelfSignedCertificateContext(elements[len(elements)-1].CertContext) {
		elements = elements[:len(elements)-1]
	}
	chainDER := make([][]byte, 0, len(elements))
	for index, element := range elements {
		if element == nil || element.CertContext == nil {
			return nil, E.New("missing certificate chain element ", index)
		}
		der := certificateContextDER(element.CertContext)
		if len(der) == 0 {
			return nil, E.New("empty certificate chain element ", index)
		}
		chainDER = append(chainDER, der)
	}
	return chainDER, nil
}

func isSelfSignedCertificateContext(ctx *windows.CertContext) bool {
	if ctx == nil || ctx.CertInfo == nil {
		return false
	}
	return bytes.Equal(certificateNameBlobBytes(ctx.CertInfo.Issuer), certificateNameBlobBytes(ctx.CertInfo.Subject))
}

func certificateNameBlobBytes(blob windows.CertNameBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	return unsafe.Slice(blob.Data, int(blob.Size))
}

func (s *Session) load() Handle {
	return Handle(s.handle.Load())
}

func (s *Session) take() Handle {
	return Handle(s.handle.Swap(0))
}

func (c *Connection) load() Handle {
	return Handle(c.handle.Load())
}

func (c *Connection) take() Handle {
	return Handle(c.handle.Swap(0))
}

func (r *Request) load() Handle {
	return Handle(r.handle.Load())
}

func (r *Request) take() Handle {
	return Handle(r.handle.Swap(0))
}

func startAsync(op string, err error) error {
	if err == nil || errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil
	}
	return wrapError(op, err)
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	handleErr := convertError(err)
	if errors.Is(handleErr, os.ErrClosed) {
		return handleErr
	}
	return E.Cause(handleErr, "winhttp: ", op)
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	var errno windows.Errno
	if errors.As(err, &errno) {
		if errno == windows.ERROR_INVALID_HANDLE {
			return netClosedError{}
		}
		if errno > errorBase && errno <= errorLast {
			return Error(errno)
		}
	}
	return err
}

type netClosedError struct{}

func (netClosedError) Error() string {
	return os.ErrClosed.Error()
}

func (netClosedError) Is(target error) bool {
	return target == os.ErrClosed
}

func (e Error) Error() string {
	var message [2048]uint16
	n, err := windows.FormatMessage(
		windows.FORMAT_MESSAGE_FROM_HMODULE|windows.FORMAT_MESSAGE_IGNORE_INSERTS|windows.FORMAT_MESSAGE_MAX_WIDTH_MASK,
		modwinhttp.Handle(),
		uint32(e),
		0,
		message[:],
		nil,
	)
	if err != nil {
		return fmt.Sprintf("WinHTTP error #%d", e)
	}
	return strings.TrimSpace(windows.UTF16ToString(message[:n]))
}

func init() {
	runtime.KeepAlive(modwinhttp)
}
