//go:build windows

package winhttp

import "golang.org/x/sys/windows"

type Handle windows.Handle

type Error uint32

type AsyncResult struct {
	Result uintptr
	Error  uint32
}

type ProxyInfo struct {
	AccessType  uint32
	Proxy       *uint16
	ProxyBypass *uint16
}

type ConnectionInfo struct {
	Protocol         uint32
	Cipher           uint32
	CipherStrength   uint32
	Hash             uint32
	HashStrength     uint32
	Exchange         uint32
	ExchangeStrength uint32
}

type CipherInfo struct {
	Version         uint32
	Protocol        uint32
	CipherSuite     uint32
	BaseCipherSuite uint32
	CipherSuiteName [64]uint16
	CipherName      [64]uint16
	CipherLen       uint32
	CipherBlockLen  uint32
	HashName        [64]uint16
	HashLen         uint32
	ExchangeName    [64]uint16
	MinExchangeLen  uint32
	MaxExchangeLen  uint32
	CertificateName [64]uint16
	KeyType         uint32
}

type SecurityInfo struct {
	ConnectionInfo ConnectionInfo
	CipherInfo     CipherInfo
}

const (
	accessTypeNoProxy    = 1
	accessTypeNamedProxy = 3

	FlagAsync  = 0x10000000
	FlagSecure = 0x00800000

	ProtocolFlagHTTP2 = 0x1
	ProtocolFlagHTTP3 = 0x2

	queryVersion        = 18
	queryStatusCode     = 19
	queryRawHeadersCRLF = 22
	queryFlagNumber     = 0x20000000

	optionContextValue            = 45
	optionProxy                   = 38
	optionProxyUsername           = 4098
	optionProxyPassword           = 4099
	optionRedirectPolicy          = 88
	optionSecureProtocols         = 84
	optionSecurityFlags           = 31
	optionDisableFeature          = 63
	optionEnableHTTPProtocol      = 133
	optionHTTPProtocolUsed        = 134
	optionHTTPProtocolRequired    = 145
	optionSecurityInfo            = 151
	optionServerCertContext       = 78
	optionServerCertChainContext  = 147
	optionDisableProxyAuthSchemes = 193
	optionDisableGlobalPooling    = 195

	RedirectPolicyNever = 0

	DisableCookies   = 1
	DisableRedirects = 2

	SecurityFlagIgnoreUnknownCA       = 0x00000100
	SecurityFlagIgnoreCertWrongUsage  = 0x00000200
	SecurityFlagIgnoreCertCNInvalid   = 0x00001000
	SecurityFlagIgnoreCertDateInvalid = 0x00002000

	ProxyDisableSchemeDigest    = 0x2
	ProxyDisableSchemeNTLM      = 0x4
	ProxyDisableSchemeKerberos  = 0x8
	ProxyDisableSchemeNegotiate = 0x10

	SecureProtocolSSL2  = 0x00000008
	SecureProtocolSSL3  = 0x00000020
	SecureProtocolTLS10 = 0x00000080
	SecureProtocolTLS11 = 0x00000200
	SecureProtocolTLS12 = 0x00000800
	SecureProtocolTLS13 = 0x00002000

	CallbackStatusHandleClosing       = 0x00000800
	CallbackStatusSecureFailure       = 0x00010000
	CallbackStatusHeadersAvailable    = 0x00020000
	CallbackStatusReadComplete        = 0x00080000
	CallbackStatusRequestError        = 0x00200000
	CallbackStatusSendRequestComplete = 0x00400000

	CallbackStatusFlagCertRevFailed        = 0x00000001
	CallbackStatusFlagCertRevoked          = 0x00000004
	CallbackStatusFlagInvalidCA            = 0x00000008
	CallbackStatusFlagCertCNInvalid        = 0x00000010
	CallbackStatusFlagCertDateInvalid      = 0x00000020
	CallbackStatusFlagCertWrongUsage       = 0x00000040
	CallbackStatusFlagSecurityChannelError = 0x80000000

	CallbackFlagHeadersAvailable    = CallbackStatusHeadersAvailable
	CallbackFlagReadComplete        = CallbackStatusReadComplete
	CallbackFlagRequestError        = CallbackStatusRequestError
	CallbackFlagSendRequestComplete = CallbackStatusSendRequestComplete
	CallbackFlagSecureFailure       = CallbackStatusSecureFailure
	CallbackFlagHandles             = 0x00000400 | CallbackStatusHandleClosing

	APIReceiveResponse = 1
	APIReadData        = 3
	APISendRequest     = 5

	errorBase                  = 12000
	ErrorInvalidURL            = Error(12005)
	ErrorUnrecognizedScheme    = Error(12006)
	ErrorNameNotResolved       = Error(12007)
	ErrorInvalidOption         = Error(12009)
	ErrorOptionNotSettable     = Error(12011)
	ErrorShutdown              = Error(12012)
	ErrorLoginFailure          = Error(12015)
	ErrorOperationCancelled    = Error(12017)
	ErrorIncorrectHandleType   = Error(12018)
	ErrorIncorrectHandleState  = Error(12019)
	ErrorTimeout               = Error(12002)
	ErrorCannotConnect         = Error(12029)
	ErrorConnectionError       = Error(12030)
	ErrorResendRequest         = Error(12032)
	ErrorSecureFailure         = Error(12175)
	ErrorSecureCertDateInvalid = Error(12037)
	ErrorSecureCertCNInvalid   = Error(12038)
	ErrorSecureInvalidCA       = Error(12045)
	ErrorSecureChannelError    = Error(12157)
	ErrorSecureInvalidCert     = Error(12169)
	ErrorSecureCertRevoked     = Error(12170)
	ErrorSecureCertWrongUsage  = Error(12179)
	ErrorHeaderNotFound        = Error(12150)
	ErrorInvalidServerResponse = Error(12152)
	ErrorHTTPProtocolMismatch  = Error(12190)
	errorLast                  = 12190

	invalidStatusCallback = ^uintptr(0)
)
