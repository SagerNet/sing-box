package schannel

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"os"
	"sync"
	"syscall"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

const clientCredentialFlags = schCredManualCredValidation | schCredNoDefaultCreds | schUseStrongCrypto

var versionCheck = sync.OnceValue(func() error {
	major, _, build := windows.RtlGetNtVersionNumbers()
	build &= 0xffff
	if major < 10 || (major == 10 && build < 17763) {
		return E.New("Windows TLS engine requires Windows build 17763 or later (Windows 10 version 1809, Windows Server 2019, or newer)")
	}
	return nil
})

// CheckPlatform returns an error when the running Windows version does not
// support the SCH_CREDENTIALS structure used by this package.
func CheckPlatform() error {
	return versionCheck()
}

type clientCredentialKey struct {
	disabledProtocols uint32
	flags             uint32
}

type clientCredential struct {
	key       clientCredentialKey
	once      sync.Once
	handle    secHandle
	tlsParams tlsParameters
	err       error
}

var clientCredentialCache sync.Map

func cachedClientCredential(minVersion, maxVersion uint16) (*clientCredential, error) {
	key := clientCredentialKey{
		disabledProtocols: disabledProtocolsMask(minVersion, maxVersion),
		flags:             clientCredentialFlags,
	}
	actual, _ := clientCredentialCache.LoadOrStore(key, &clientCredential{key: key})
	credential := actual.(*clientCredential)
	credential.once.Do(func() {
		credential.err = credential.acquire()
	})
	if credential.err != nil {
		clientCredentialCache.Delete(key)
		return nil, credential.err
	}
	return credential, nil
}

func (c *clientCredential) acquire() error {
	c.tlsParams.grbitDisabledProtocols = c.key.disabledProtocols
	sch := schCredentials{
		dwVersion:      schCredentialsVersion,
		dwFlags:        c.key.flags,
		cTlsParameters: 1,
		pTlsParameters: &c.tlsParams,
	}
	pkg, err := windows.UTF16PtrFromString(unispNameW)
	if err != nil {
		return err
	}
	var expiry windows.Filetime
	status := sspiAcquireCredentialsHandle(
		nil,
		pkg,
		secPkgCredOutbound,
		nil,
		unsafe.Pointer(&sch),
		0,
		0,
		&c.handle,
		&expiry,
	)
	if status != secEOK {
		return sspiError("AcquireCredentialsHandle", status)
	}
	return nil
}

// ClientContext owns the per-connection Schannel security context and drives
// it through handshake and application-data phases.
type ClientContext struct {
	credential *clientCredential
	handle     secHandle
	targetName *uint16

	// alpnBuffer is the SEC_APPLICATION_PROTOCOLS blob; kept alive for the
	// duration of the first handshake call.
	alpnBuffer []byte

	firstCall bool
	valid     bool
}

// NewClientContext allocates a new client context, reuses the Schannel
// credential handle for the supplied TLS version bounds, and advertises ALPN
// protocols through an SECBUFFER_APPLICATION_PROTOCOLS buffer on the first
// handshake call.
func NewClientContext(minVersion, maxVersion uint16, serverName string, alpn []string) (*ClientContext, error) {
	if minVersion != 0 && maxVersion != 0 && minVersion > maxVersion {
		return nil, os.ErrInvalid
	}
	err := CheckPlatform()
	if err != nil {
		return nil, err
	}
	targetName, err := windows.UTF16PtrFromString(serverName)
	if err != nil {
		return nil, err
	}
	credential, err := cachedClientCredential(minVersion, maxVersion)
	if err != nil {
		return nil, err
	}
	c := &ClientContext{
		credential: credential,
		targetName: targetName,
		firstCall:  true,
	}
	if len(alpn) > 0 {
		c.alpnBuffer, err = encodeAlpnBuffer(alpn)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Close releases the per-connection security context. Safe to call multiple
// times.
func (c *ClientContext) Close() {
	if c == nil {
		return
	}
	if c.valid {
		sspiDeleteSecurityContext(&c.handle)
		c.valid = false
		c.handle = secHandle{}
	}
}

type StepResult struct {
	// Output must be written to the peer verbatim before the next Step call.
	// When Done is true, leftover input[Consumed:] is the first application
	// ciphertext — not more handshake bytes.
	Output     []byte
	Consumed   int
	Done       bool
	Incomplete bool
}

// Step drives one handshake iteration. Input may be nil on the first call.
// Callers must write Output to the peer, append more peer bytes when
// Incomplete is true, and loop until Done is true.
func (c *ClientContext) Step(input []byte) (StepResult, error) {
	var inputDesc *secBufferDesc
	var inputBufs [2]secBuffer
	if c.firstCall {
		if len(c.alpnBuffer) > 0 {
			inputBufs[0].bufferType = secbufferApplicationProtocols
			inputBufs[0].cbBuffer = uint32(len(c.alpnBuffer))
			inputBufs[0].pvBuffer = &c.alpnBuffer[0]
			inputDesc = &secBufferDesc{
				ulVersion: secbufferVersion,
				cBuffers:  1,
				pBuffers:  &inputBufs[0],
			}
		}
	} else {
		if len(input) == 0 {
			return StepResult{}, E.New("schannel: empty handshake input after first step")
		}
		inputBufs[0].bufferType = secbufferToken
		inputBufs[0].cbBuffer = uint32(len(input))
		inputBufs[0].pvBuffer = &input[0]
		inputBufs[1].bufferType = secbufferEmpty
		inputDesc = &secBufferDesc{
			ulVersion: secbufferVersion,
			cBuffers:  2,
			pBuffers:  &inputBufs[0],
		}
	}

	result, terminal, err := c.runInitializeSecurityContext(inputDesc, "InitializeSecurityContext")
	if err != nil || terminal {
		return result, err
	}

	switch {
	case c.firstCall:
		result.Consumed = 0
	case inputBufs[1].bufferType == secbufferExtra && inputBufs[1].cbBuffer > 0:
		consumed, extraErr := consumedFromExtra(&inputBufs[1], len(input))
		if extraErr != nil {
			return result, extraErr
		}
		result.Consumed = consumed
	default:
		result.Consumed = len(input)
	}

	c.firstCall = false
	c.alpnBuffer = nil
	return result, nil
}

// StreamSizes must be called after Step returns Done=true.
func (c *ClientContext) StreamSizes() (header, trailer, maxMessage uint32, err error) {
	var sizes secPkgContextStreamSizes
	status := sspiQueryContextAttributes(&c.handle, secpkgAttrStreamSizes, unsafe.Pointer(&sizes))
	if status != secEOK {
		return 0, 0, 0, sspiError("QueryContextAttributes(stream sizes)", status)
	}
	return sizes.cbHeader, sizes.cbTrailer, sizes.cbMaximumMessage, nil
}

// Encrypt wraps a plaintext chunk into a TLS record using the supplied
// backing buffer which must have room for header + plaintext + trailer bytes.
// Plaintext is copied into buffer starting at `header` offset before calling
// EncryptMessage. Returns the encrypted record as a slice into buffer.
func (c *ClientContext) Encrypt(header, trailer uint32, plaintext []byte, buffer []byte) ([]byte, error) {
	if len(buffer) < int(header)+len(plaintext)+int(trailer) {
		return nil, E.New("schannel: encrypt buffer too small")
	}
	copy(buffer[header:], plaintext)
	headerPtr := &buffer[0]
	dataPtr := &buffer[header]
	trailerPtr := &buffer[int(header)+len(plaintext)]

	bufs := [4]secBuffer{
		{cbBuffer: header, bufferType: secbufferStreamHeader, pvBuffer: headerPtr},
		{cbBuffer: uint32(len(plaintext)), bufferType: secbufferData, pvBuffer: dataPtr},
		{cbBuffer: trailer, bufferType: secbufferStreamTrailer, pvBuffer: trailerPtr},
		{bufferType: secbufferEmpty},
	}
	desc := secBufferDesc{
		ulVersion: secbufferVersion,
		cBuffers:  4,
		pBuffers:  &bufs[0],
	}
	status := sspiEncryptMessage(&c.handle, 0, &desc, 0)
	if status != secEOK {
		return nil, sspiError("EncryptMessage", status)
	}
	total := int(bufs[0].cbBuffer + bufs[1].cbBuffer + bufs[2].cbBuffer)
	return buffer[:total], nil
}

type DecryptResult struct {
	// Plaintext aliases memory inside the input buffer passed to Decrypt;
	// callers must copy before the next Decrypt call reuses that buffer.
	Plaintext []byte
	// ConsumedTotal is the number of input bytes Schannel consumed, i.e.
	// input[ConsumedTotal:] are unprocessed leftover ciphertext.
	ConsumedTotal int
	// RenegotiateToken aliases the post-handshake token that must be fed back
	// through InitializeSecurityContext after SEC_I_RENEGOTIATE.
	RenegotiateToken []byte
	Incomplete       bool
	Renegotiate      bool
	Expired          bool
}

// Decrypt processes a chunk of TLS ciphertext in-place. The returned Plaintext
// aliases memory inside input until the next Decrypt call; callers must copy
// the bytes they want to keep.
func (c *ClientContext) Decrypt(input []byte) (DecryptResult, error) {
	var result DecryptResult
	if len(input) == 0 {
		result.Incomplete = true
		return result, nil
	}
	bufs := [4]secBuffer{
		{cbBuffer: uint32(len(input)), bufferType: secbufferData, pvBuffer: &input[0]},
		{bufferType: secbufferEmpty},
		{bufferType: secbufferEmpty},
		{bufferType: secbufferEmpty},
	}
	desc := secBufferDesc{
		ulVersion: secbufferVersion,
		cBuffers:  4,
		pBuffers:  &bufs[0],
	}
	status := sspiDecryptMessage(&c.handle, &desc, 0, nil)
	switch status {
	case secEOK:
	case secEIncompleteMessage:
		result.Incomplete = true
		return result, nil
	case secIContextExpired:
		result.Expired = true
		return result, nil
	case secIRenegotiate:
		result.Renegotiate = true
	default:
		return result, sspiError("DecryptMessage", status)
	}
	return parseDecryptResult(input, bufs[:], result.Renegotiate)
}

// PostHandshake processes a TLS 1.3 post-handshake message
// (NewSessionTicket, KeyUpdate) after DecryptMessage returned
// SEC_I_RENEGOTIATE. Pass the token preserved from Decrypt on the first call;
// pass more peer bytes on subsequent calls when Incomplete.
func (c *ClientContext) PostHandshake(input []byte) (StepResult, error) {
	var inputDesc *secBufferDesc
	var inputBufs [2]secBuffer
	if len(input) > 0 {
		inputBufs[0].bufferType = secbufferToken
		inputBufs[0].cbBuffer = uint32(len(input))
		inputBufs[0].pvBuffer = &input[0]
		inputBufs[1].bufferType = secbufferEmpty
		inputDesc = &secBufferDesc{
			ulVersion: secbufferVersion,
			cBuffers:  2,
			pBuffers:  &inputBufs[0],
		}
	}

	result, terminal, err := c.runInitializeSecurityContext(inputDesc, "InitializeSecurityContext(post-handshake)")
	if err != nil || terminal {
		return result, err
	}

	if len(input) > 0 && inputBufs[1].bufferType == secbufferExtra && inputBufs[1].cbBuffer > 0 {
		consumed, extraErr := consumedFromExtra(&inputBufs[1], len(input))
		if extraErr != nil {
			return result, extraErr
		}
		result.Consumed = consumed
	} else {
		result.Consumed = len(input)
	}
	return result, nil
}

func parseDecryptResult(input []byte, bufs []secBuffer, renegotiate bool) (DecryptResult, error) {
	var result DecryptResult
	var dataBuffer, extraBuffer *secBuffer
	for index := range bufs {
		switch bufs[index].bufferType {
		case secbufferData:
			dataBuffer = &bufs[index]
		case secbufferExtra:
			extraBuffer = &bufs[index]
		}
	}
	if dataBuffer != nil && dataBuffer.cbBuffer > 0 && dataBuffer.pvBuffer != nil {
		result.Plaintext = unsafe.Slice(dataBuffer.pvBuffer, int(dataBuffer.cbBuffer))
	}
	if extraBuffer != nil && extraBuffer.cbBuffer > 0 {
		consumed, err := consumedFromExtra(extraBuffer, len(input))
		if err != nil {
			return result, err
		}
		result.ConsumedTotal = consumed
	} else {
		result.ConsumedTotal = len(input)
	}
	if renegotiate {
		result.Renegotiate = true
		if extraBuffer != nil && extraBuffer.cbBuffer > 0 {
			result.RenegotiateToken = input[result.ConsumedTotal:]
		} else {
			result.RenegotiateToken = input
		}
	}
	return result, nil
}

// ApplicationProtocol returns the empty string when ALPN was not negotiated.
func (c *ClientContext) ApplicationProtocol() (string, error) {
	var info secPkgContextApplicationProtocol
	status := sspiQueryContextAttributes(&c.handle, secpkgAttrApplicationProtocol, unsafe.Pointer(&info))
	if status != secEOK {
		return "", sspiError("QueryContextAttributes(application protocol)", status)
	}
	if info.protoNegoStatus != secApplicationProtocolNegotiationStatusSuccess {
		return "", nil
	}
	size := int(info.protocolIDSize)
	if size > len(info.protocolID) {
		return "", E.New("schannel: invalid ALPN protocol size")
	}
	return string(info.protocolID[:size]), nil
}

// ConnectionInfo reports the negotiated TLS version and cipher suite.
// cipherSuite may be zero when the Windows build does not return a
// mappable cipher name.
func (c *ClientContext) ConnectionInfo() (version, cipherSuite uint16, err error) {
	var info secPkgContextConnectionInfo
	status := sspiQueryContextAttributes(&c.handle, secpkgAttrConnectionInfo, unsafe.Pointer(&info))
	if status != secEOK {
		return 0, 0, sspiError("QueryContextAttributes(connection info)", status)
	}
	version = sspProtocolToTLSVersion(info.dwProtocol)

	var cipherInfo secPkgContextCipherInfo
	cipherInfo.dwVersion = 1
	status = sspiQueryContextAttributes(&c.handle, secpkgAttrCipherInfo, unsafe.Pointer(&cipherInfo))
	if status == secEOK {
		cipherSuite = cipherSuiteID(windows.UTF16ToString(cipherInfo.szCipherSuite[:]))
	}
	return version, cipherSuite, nil
}

func cipherSuiteID(name string) uint16 {
	for _, suite := range tls.CipherSuites() {
		if suite.Name == name {
			return suite.ID
		}
	}
	for _, suite := range tls.InsecureCipherSuites() {
		if suite.Name == name {
			return suite.ID
		}
	}
	return 0
}

// RemoteCertificateChain returns freshly allocated DER bytes ordered
// leaf → intermediates.
func (c *ClientContext) RemoteCertificateChain() ([][]byte, error) {
	var leaf *windows.CertContext
	status := sspiQueryContextAttributes(&c.handle, secpkgAttrRemoteCertContext, unsafe.Pointer(&leaf))
	if status != secEOK {
		return nil, sspiError("QueryContextAttributes(remote cert context)", status)
	}
	if leaf == nil {
		return nil, nil
	}
	defer windows.CertFreeCertificateContext(leaf)

	chain, err := buildCertChainDER(leaf)
	if err != nil {
		return [][]byte{certContextDER(leaf)}, nil
	}
	return chain, nil
}

const handshakeContextReq = iscReqSequenceDetect |
	iscReqReplayDetect |
	iscReqConfidentiality |
	iscReqAllocateMemory |
	iscReqStream |
	iscReqUseSuppliedCreds |
	iscReqManualCredValidation |
	iscReqExtendedError

// runInitializeSecurityContext returns terminal=true when the result is
// final (error or more-data-needed), signalling that the caller must skip
// extra-buffer post-processing.
func (c *ClientContext) runInitializeSecurityContext(inputDesc *secBufferDesc, opLabel string) (StepResult, bool, error) {
	var outputBufs [1]secBuffer
	outputBufs[0].bufferType = secbufferToken
	outputDesc := secBufferDesc{
		ulVersion: secbufferVersion,
		cBuffers:  1,
		pBuffers:  &outputBufs[0],
	}
	var ctxIn *secHandle
	if c.valid {
		ctxIn = &c.handle
	}
	var contextAttr uint32
	var expiry windows.Filetime
	status := sspiInitializeSecurityContext(
		&c.credential.handle,
		ctxIn,
		c.targetName,
		handshakeContextReq,
		0,
		0,
		inputDesc,
		0,
		&c.handle,
		&outputDesc,
		&contextAttr,
		&expiry,
	)

	switch status {
	case secEOK, secICompleteNeeded, secICompleteAndContinue, secIContinueNeeded:
		c.valid = true
	}
	if status == secICompleteNeeded || status == secICompleteAndContinue {
		completeStatus := sspiCompleteAuthToken(&c.handle, &outputDesc)
		if completeStatus != secEOK {
			if outputBufs[0].pvBuffer != nil {
				sspiFreeContextBuffer(outputBufs[0].pvBuffer)
			}
			return StepResult{}, true, sspiError("CompleteAuthToken", completeStatus)
		}
	}

	var result StepResult
	if outputBufs[0].cbBuffer > 0 && outputBufs[0].pvBuffer != nil {
		result.Output = unsafeSliceCopy(outputBufs[0].pvBuffer, int(outputBufs[0].cbBuffer))
		sspiFreeContextBuffer(outputBufs[0].pvBuffer)
	}

	switch status {
	case secEOK, secICompleteNeeded:
		result.Done = true
		return result, false, nil
	case secIContinueNeeded, secICompleteAndContinue:
		return result, false, nil
	case secEIncompleteMessage:
		c.valid = true
		result.Incomplete = true
		return result, true, nil
	default:
		return result, true, sspiError(opLabel, status)
	}
}

func consumedFromExtra(extraBuf *secBuffer, inputLen int) (int, error) {
	extraLen := int(extraBuf.cbBuffer)
	if extraLen > inputLen {
		return 0, E.New("schannel: SECBUFFER_EXTRA exceeds input length")
	}
	return inputLen - extraLen, nil
}

func disabledProtocolsMask(minVersion, maxVersion uint16) uint32 {
	allowed := uint32(0)
	versions := []struct {
		id   uint16
		mask uint32
	}{
		{tls.VersionTLS10, spProtTLS10Client},
		{tls.VersionTLS11, spProtTLS11Client},
		{tls.VersionTLS12, spProtTLS12Client},
		{tls.VersionTLS13, spProtTLS13Client},
	}
	effectiveMin := minVersion
	if effectiveMin == 0 {
		effectiveMin = tls.VersionTLS12
		if maxVersion != 0 && maxVersion < tls.VersionTLS12 {
			effectiveMin = versions[0].id
		}
	}
	effectiveMax := maxVersion
	if effectiveMax == 0 {
		effectiveMax = tls.VersionTLS13
	}
	for _, v := range versions {
		if v.id >= effectiveMin && v.id <= effectiveMax {
			allowed |= v.mask
		}
	}
	if allowed == 0 {
		return 0
	}
	return spProtAllTLSClients &^ allowed
}

func sspProtocolToTLSVersion(sp uint32) uint16 {
	switch {
	case sp&spProtTLS13Client != 0:
		return tls.VersionTLS13
	case sp&spProtTLS12Client != 0:
		return tls.VersionTLS12
	case sp&spProtTLS11Client != 0:
		return tls.VersionTLS11
	case sp&spProtTLS10Client != 0:
		return tls.VersionTLS10
	}
	return 0
}

func encodeAlpnBuffer(protocols []string) ([]byte, error) {
	var protoList []byte
	for _, proto := range protocols {
		if len(proto) == 0 || len(proto) > 255 {
			return nil, E.New("schannel: invalid ALPN protocol: ", proto)
		}
		protoList = append(protoList, byte(len(proto)))
		protoList = append(protoList, []byte(proto)...)
	}
	if len(protoList) > 0xFFFF {
		return nil, E.New("schannel: ALPN list too long")
	}
	// Layout:
	//   uint32 ProtocolListsSize
	//     uint32 ProtoNegoExt
	//     uint16 ProtocolListSize
	//     bytes  ProtocolList
	inner := 4 + 2 + len(protoList)
	buffer := make([]byte, 4+inner)
	binary.LittleEndian.PutUint32(buffer[0:4], uint32(inner))
	binary.LittleEndian.PutUint32(buffer[4:8], secApplicationProtocolNegotiationExtALPN)
	binary.LittleEndian.PutUint16(buffer[8:10], uint16(len(protoList)))
	copy(buffer[10:], protoList)
	return buffer, nil
}

func unsafeSliceCopy(ptr *byte, size int) []byte {
	if ptr == nil || size <= 0 {
		return nil
	}
	out := make([]byte, size)
	copy(out, unsafe.Slice(ptr, size))
	return out
}

func certContextDER(ctx *windows.CertContext) []byte {
	if ctx == nil || ctx.EncodedCert == nil || ctx.Length == 0 {
		return nil
	}
	out := make([]byte, ctx.Length)
	copy(out, unsafe.Slice(ctx.EncodedCert, int(ctx.Length)))
	return out
}

func buildCertChainDER(leaf *windows.CertContext) ([][]byte, error) {
	var chainPara windows.CertChainPara
	chainPara.Size = uint32(unsafe.Sizeof(chainPara))
	var chainCtx *windows.CertChainContext
	err := windows.CertGetCertificateChain(0, leaf, nil, leaf.Store, &chainPara, 0, 0, &chainCtx)
	if err != nil {
		return nil, err
	}
	defer windows.CertFreeCertificateChain(chainCtx)
	return extractCertChainDER(chainCtx)
}

func extractCertChainDER(chainCtx *windows.CertChainContext) ([][]byte, error) {
	if chainCtx == nil || chainCtx.ChainCount == 0 || chainCtx.Chains == nil {
		return nil, E.New("schannel: empty certificate chain")
	}
	chains := unsafe.Slice(chainCtx.Chains, int(chainCtx.ChainCount))
	chain := chains[0]
	if chain == nil || chain.NumElements == 0 || chain.Elements == nil {
		return nil, E.New("schannel: empty certificate chain")
	}
	elements := unsafe.Slice(chain.Elements, int(chain.NumElements))
	if len(elements) > 1 &&
		chain.TrustStatus.ErrorStatus&windows.CERT_TRUST_IS_PARTIAL_CHAIN == 0 &&
		isSelfSignedCertContext(elements[len(elements)-1].CertContext) {
		elements = elements[:len(elements)-1]
	}
	derChain := make([][]byte, 0, len(elements))
	for index, element := range elements {
		if element == nil || element.CertContext == nil {
			return nil, E.New("schannel: missing certificate chain element ", index)
		}
		der := certContextDER(element.CertContext)
		if len(der) == 0 {
			return nil, E.New("schannel: empty certificate chain element ", index)
		}
		derChain = append(derChain, der)
	}
	return derChain, nil
}

func isSelfSignedCertContext(ctx *windows.CertContext) bool {
	if ctx == nil || ctx.CertInfo == nil {
		return false
	}
	return bytes.Equal(
		certNameBlobBytes(ctx.CertInfo.Issuer),
		certNameBlobBytes(ctx.CertInfo.Subject),
	)
}

func certNameBlobBytes(blob windows.CertNameBlob) []byte {
	if blob.Size == 0 || blob.Data == nil {
		return nil
	}
	return unsafe.Slice(blob.Data, int(blob.Size))
}

func sspiError(where string, status syscall.Errno) error {
	return E.New("schannel: ", where, ": ", formatStatus(status))
}

var statusNames = map[syscall.Errno]string{
	secEUnsupportedFunc:           "SEC_E_UNSUPPORTED_FUNCTION",
	secEInternalError:             "SEC_E_INTERNAL_ERROR",
	secEInvalidToken:              "SEC_E_INVALID_TOKEN",
	secELogonDenied:               "SEC_E_LOGON_DENIED",
	secEMessageAltered:            "SEC_E_MESSAGE_ALTERED",
	secENoAuthenticatingAuthority: "SEC_E_NO_AUTHENTICATING_AUTHORITY",
	secEContextExpired:            "SEC_E_CONTEXT_EXPIRED",
	secEIncompleteMessage:         "SEC_E_INCOMPLETE_MESSAGE",
	secEIncompleteCreds:           "SEC_E_INCOMPLETE_CREDENTIALS",
	secEBufferTooSmall:            "SEC_E_BUFFER_TOO_SMALL",
	secEWrongPrincipal:            "SEC_E_WRONG_PRINCIPAL",
	secEIllegalMessage:            "SEC_E_ILLEGAL_MESSAGE",
	secECertUnknown:               "SEC_E_CERT_UNKNOWN",
	secECertExpired:               "SEC_E_CERT_EXPIRED",
	secEAlgorithmMismatch:         "SEC_E_ALGORITHM_MISMATCH",
}

func formatStatus(status syscall.Errno) string {
	name, loaded := statusNames[status]
	if !loaded {
		return status.Error()
	}
	return name + ": " + status.Error()
}
