//go:build windows

package main

import (
	"bytes"
	"crypto/x509"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

var (
	winTrustLibrary                               = windows.NewLazySystemDLL("wintrust.dll")
	winTrustProviderDataFromStateDataProcedure    = winTrustLibrary.NewProc("WTHelperProvDataFromStateData")
	winTrustProviderSignerFromChainProcedure      = winTrustLibrary.NewProc("WTHelperGetProvSignerFromChain")
	winTrustProviderCertificateFromChainProcedure = winTrustLibrary.NewProc("WTHelperGetProvCertFromChain")
)

type cryptProviderCertificate struct {
	structureSize      uint32
	certificateContext *windows.CertContext
}

func authenticodeSigner(path string, file windows.Handle) ([]byte, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	fileInformation := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: pathPointer,
		File:     file,
	}
	trustData := windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInformation),
		ProvFlags: windows.WTD_CACHE_ONLY_URL_RETRIEVAL |
			windows.WTD_REVOCATION_CHECK_NONE |
			windows.WTD_DISABLE_MD2_MD4,
		UIContext: windows.WTD_UICONTEXT_EXECUTE,
	}
	trustError := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &trustData)
	if trustError != nil && !E.IsMulti(
		trustError,
		windows.Errno(windows.CERT_E_UNTRUSTEDROOT),
		windows.Errno(windows.CERT_E_CHAINING),
	) {
		trustData.StateAction = windows.WTD_STATEACTION_CLOSE
		windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &trustData)
		return nil, E.Cause(trustError, "verify Authenticode signature")
	}
	signer, signerError := verifiedSignerCertificate(trustData.StateData)
	trustData.StateAction = windows.WTD_STATEACTION_CLOSE
	closeError := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &trustData)
	if signerError != nil {
		return nil, signerError
	}
	if closeError != nil {
		return nil, E.Cause(closeError, "close Authenticode verification")
	}
	certificate, err := validateCodeSigningCertificate(signer)
	if err != nil {
		return nil, err
	}
	if trustError != nil {
		err = validateUntrustedSelfSignedCertificate(certificate, time.Now())
		if err != nil {
			return nil, err
		}
	}
	return signer, nil
}

func verifiedSignerCertificate(stateData windows.Handle) ([]byte, error) {
	providerData, _, _ := winTrustProviderDataFromStateDataProcedure.Call(uintptr(stateData))
	if providerData == 0 {
		return nil, E.New("missing Authenticode provider data")
	}
	providerSigner, _, _ := winTrustProviderSignerFromChainProcedure.Call(providerData, 0, 0, 0)
	if providerSigner == 0 {
		return nil, E.New("missing Authenticode provider signer")
	}
	providerCertificate, _, _ := winTrustProviderCertificateFromChainProcedure.Call(providerSigner, 0)
	if providerCertificate == 0 {
		return nil, E.New("missing Authenticode provider certificate")
	}
	certificateContext := (*cryptProviderCertificate)(unsafe.Pointer(providerCertificate)).certificateContext
	if certificateContext == nil {
		return nil, E.New("empty Authenticode signer certificate context")
	}
	if certificateContext.Length == 0 || certificateContext.EncodedCert == nil {
		return nil, E.New("empty Authenticode signer certificate")
	}
	encodedCertificate := unsafe.Slice(certificateContext.EncodedCert, int(certificateContext.Length))
	return append([]byte(nil), encodedCertificate...), nil
}

func validateCodeSigningCertificate(encodedCertificate []byte) (*x509.Certificate, error) {
	certificate, err := x509.ParseCertificate(encodedCertificate)
	if err != nil {
		return nil, E.Cause(err, "parse Authenticode signer certificate")
	}
	for _, usage := range certificate.ExtKeyUsage {
		if usage == x509.ExtKeyUsageCodeSigning || usage == x509.ExtKeyUsageAny {
			return certificate, nil
		}
	}
	return nil, E.New("Authenticode signer certificate is not valid for code signing")
}

func validateUntrustedSelfSignedCertificate(certificate *x509.Certificate, currentTime time.Time) error {
	if !bytes.Equal(certificate.RawSubject, certificate.RawIssuer) {
		return E.New("untrusted Authenticode signer certificate is not self-signed")
	}
	err := certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature)
	if err != nil {
		return E.Cause(err, "verify untrusted Authenticode signer self-signature")
	}
	if currentTime.Before(certificate.NotBefore) || currentTime.After(certificate.NotAfter) {
		return E.New("untrusted Authenticode signer certificate is not currently valid")
	}
	return nil
}
