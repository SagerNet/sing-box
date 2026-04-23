package schannel

import (
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	unispNameW = "Microsoft Unified Security Protocol Provider"

	schCredentialsVersion = 5

	secPkgCredOutbound = 2

	iscReqSequenceDetect       = 0x00000008
	iscReqReplayDetect         = 0x00000004
	iscReqConfidentiality      = 0x00000010
	iscReqAllocateMemory       = 0x00000100
	iscReqStream               = 0x00008000
	iscReqUseSuppliedCreds     = 0x00000080
	iscReqManualCredValidation = 0x00080000
	iscReqExtendedError        = 0x00004000

	secbufferEmpty                = 0
	secbufferData                 = 1
	secbufferToken                = 2
	secbufferExtra                = 5
	secbufferStreamTrailer        = 6
	secbufferStreamHeader         = 7
	secbufferApplicationProtocols = 18
	secbufferVersion              = 0

	secApplicationProtocolNegotiationExtALPN = 2

	secApplicationProtocolNegotiationStatusSuccess = 1

	schCredManualCredValidation = 0x00000008
	schCredNoDefaultCreds       = 0x00000010
	schUseStrongCrypto          = 0x00400000

	spProtTLS10Client = 0x00000080
	spProtTLS11Client = 0x00000200
	spProtTLS12Client = 0x00000800
	spProtTLS13Client = 0x00002000

	spProtAllTLSClients = spProtTLS10Client | spProtTLS11Client | spProtTLS12Client | spProtTLS13Client

	secpkgAttrStreamSizes         = 4
	secpkgAttrConnectionInfo      = 0x5A
	secpkgAttrApplicationProtocol = 0x23
	secpkgAttrCipherInfo          = 0x64
	secpkgAttrRemoteCertContext   = 0x53
)

const (
	secEOK                        = syscall.Errno(windows.SEC_E_OK)
	secICompleteNeeded            = syscall.Errno(windows.SEC_I_COMPLETE_NEEDED)
	secICompleteAndContinue       = syscall.Errno(windows.SEC_I_COMPLETE_AND_CONTINUE)
	secIContinueNeeded            = syscall.Errno(windows.SEC_I_CONTINUE_NEEDED)
	secIContextExpired            = syscall.Errno(windows.SEC_I_CONTEXT_EXPIRED)
	secIRenegotiate               = syscall.Errno(windows.SEC_I_RENEGOTIATE)
	secEIncompleteMessage         = syscall.Errno(windows.SEC_E_INCOMPLETE_MESSAGE)
	secEIncompleteCreds           = syscall.Errno(windows.SEC_E_INCOMPLETE_CREDENTIALS)
	secEBufferTooSmall            = syscall.Errno(windows.SEC_E_BUFFER_TOO_SMALL)
	secEMessageAltered            = syscall.Errno(windows.SEC_E_MESSAGE_ALTERED)
	secEContextExpired            = syscall.Errno(windows.SEC_E_CONTEXT_EXPIRED)
	secEUnsupportedFunc           = syscall.Errno(windows.SEC_E_UNSUPPORTED_FUNCTION)
	secEInvalidToken              = syscall.Errno(windows.SEC_E_INVALID_TOKEN)
	secELogonDenied               = syscall.Errno(windows.SEC_E_LOGON_DENIED)
	secEIllegalMessage            = syscall.Errno(windows.SEC_E_ILLEGAL_MESSAGE)
	secEWrongPrincipal            = syscall.Errno(windows.SEC_E_WRONG_PRINCIPAL)
	secECertUnknown               = syscall.Errno(windows.SEC_E_CERT_UNKNOWN)
	secECertExpired               = syscall.Errno(windows.SEC_E_CERT_EXPIRED)
	secEAlgorithmMismatch         = syscall.Errno(windows.SEC_E_ALGORITHM_MISMATCH)
	secEInternalError             = syscall.Errno(windows.SEC_E_INTERNAL_ERROR)
	secENoAuthenticatingAuthority = syscall.Errno(windows.SEC_E_NO_AUTHENTICATING_AUTHORITY)
)

type secHandle struct {
	lower uintptr
	upper uintptr
}

type secBuffer struct {
	cbBuffer   uint32
	bufferType uint32
	pvBuffer   *byte
}

type secBufferDesc struct {
	ulVersion uint32
	cBuffers  uint32
	pBuffers  *secBuffer
}

type schCredentials struct {
	dwVersion         uint32
	dwCredFormat      uint32
	cCreds            uint32
	paCred            uintptr
	hRootStore        windows.Handle
	cMappers          uint32
	aphMappers        uintptr
	dwSessionLifespan uint32
	dwFlags           uint32
	cTlsParameters    uint32
	pTlsParameters    *tlsParameters
}

type tlsParameters struct {
	cAlpnIds               uint32
	rgstrAlpnIds           uintptr
	grbitDisabledProtocols uint32
	cDisabledCrypto        uint32
	pDisabledCrypto        uintptr
	dwFlags                uint32
}

type secPkgContextStreamSizes struct {
	cbHeader         uint32
	cbTrailer        uint32
	cbMaximumMessage uint32
	cBuffers         uint32
	cbBlockSize      uint32
}

type secPkgContextConnectionInfo struct {
	dwProtocol       uint32
	aiCipher         uint32
	dwCipherStrength uint32
	aiHash           uint32
	dwHashStrength   uint32
	aiExch           uint32
	dwExchStrength   uint32
}

type secPkgContextApplicationProtocol struct {
	protoNegoStatus uint32
	protoNegoExt    uint32
	protocolIDSize  byte
	protocolID      [255]byte
}

type secPkgContextCipherInfo struct {
	dwVersion         uint32
	dwProtocol        uint32
	dwCipherSuite     uint32
	dwBaseCipherSuite uint32
	szCipherSuite     [64]uint16
	szCipher          [64]uint16
	dwCipherLen       uint32
	dwCipherBlockLen  uint32
	szHash            [64]uint16
	dwHashLen         uint32
	szExchange        [64]uint16
	dwMinExchangeLen  uint32
	dwMaxExchangeLen  uint32
	szCertificate     [64]uint16
	dwKeyType         uint32
}
