//go:build windows

package schannel

import (
	"bytes"
	"crypto/tls"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestExtractCertChainDERExcludesSelfSignedRoot(t *testing.T) {
	leaf := certContextForTest([]byte("leaf"), []byte("intermediate"), []byte("leaf"))
	intermediate := certContextForTest([]byte("intermediate"), []byte("root"), []byte("intermediate"))
	root := certContextForTest([]byte("root"), []byte("root"), []byte("root"))

	chainCtx := certChainContextForTest(leaf, intermediate, root)
	derChain, err := extractCertChainDER(chainCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(derChain) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(derChain))
	}
	if !bytes.Equal(derChain[0], []byte("leaf")) {
		t.Fatalf("unexpected leaf certificate: %q", string(derChain[0]))
	}
	if !bytes.Equal(derChain[1], []byte("intermediate")) {
		t.Fatalf("unexpected intermediate certificate: %q", string(derChain[1]))
	}
}

func TestExtractCertChainDERKeepsLastIntermediateWithoutRoot(t *testing.T) {
	leaf := certContextForTest([]byte("leaf"), []byte("intermediate"), []byte("leaf"))
	intermediate := certContextForTest([]byte("intermediate"), []byte("root"), []byte("intermediate"))

	chainCtx := certChainContextForTest(leaf, intermediate)
	derChain, err := extractCertChainDER(chainCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(derChain) != 2 {
		t.Fatalf("expected 2 certificates, got %d", len(derChain))
	}
	if !bytes.Equal(derChain[1], []byte("intermediate")) {
		t.Fatalf("unexpected last certificate: %q", string(derChain[1]))
	}
}

func TestDisabledProtocolsMask(t *testing.T) {
	testCases := []struct {
		name       string
		minVersion uint16
		maxVersion uint16
		want       uint32
	}{
		{
			name: "default range",
			want: spProtAllTLSClients &^ (spProtTLS12Client | spProtTLS13Client),
		},
		{
			name:       "default minimum with explicit max",
			maxVersion: tls.VersionTLS12,
			want:       spProtAllTLSClients &^ spProtTLS12Client,
		},
		{
			name:       "explicit tls10 range",
			minVersion: tls.VersionTLS10,
			maxVersion: tls.VersionTLS13,
			want:       0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := disabledProtocolsMask(testCase.minVersion, testCase.maxVersion)
			if got != testCase.want {
				t.Fatalf("disabledProtocolsMask(%#x, %#x) = %#x, want %#x", testCase.minVersion, testCase.maxVersion, got, testCase.want)
			}
		})
	}
}

func TestClientCredentialCacheReusesVersionRange(t *testing.T) {
	if err := CheckPlatform(); err != nil {
		t.Skip(err)
	}
	first, err := NewClientContext(tls.VersionTLS12, tls.VersionTLS13, "localhost", []string{"h2"})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewClientContext(tls.VersionTLS12, tls.VersionTLS13, "example.com", []string{"http/1.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if first.credential != second.credential {
		t.Fatal("expected same TLS version range to reuse credential")
	}

	tls12Only, err := NewClientContext(tls.VersionTLS12, tls.VersionTLS12, "localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tls12Only.Close()
	if first.credential == tls12Only.credential {
		t.Fatal("expected distinct TLS version range to use a distinct credential")
	}
	if first.credential.key.disabledProtocols != disabledProtocolsMask(tls.VersionTLS12, tls.VersionTLS13) {
		t.Fatalf("unexpected cached disabled protocol mask: %#x", first.credential.key.disabledProtocols)
	}
}

func TestParseDecryptResultKeepsRenegotiateExtraToken(t *testing.T) {
	input := []byte("plain-ticket")
	result, err := parseDecryptResult(input, []secBuffer{
		{bufferType: secbufferExtra, cbBuffer: 6},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renegotiate {
		t.Fatal("expected Renegotiate to be true")
	}
	if result.ConsumedTotal != len(input)-6 {
		t.Fatalf("unexpected consumed total: %d", result.ConsumedTotal)
	}
	if !bytes.Equal(result.RenegotiateToken, []byte("ticket")) {
		t.Fatalf("unexpected renegotiate token: %q", string(result.RenegotiateToken))
	}
}

func TestParseDecryptResultKeepsRenegotiateWholeBufferWithoutExtra(t *testing.T) {
	input := []byte("ticket")
	result, err := parseDecryptResult(input, []secBuffer{
		{bufferType: secbufferData, cbBuffer: uint32(len(input)), pvBuffer: &input[0]},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renegotiate {
		t.Fatal("expected Renegotiate to be true")
	}
	if result.ConsumedTotal != len(input) {
		t.Fatalf("unexpected consumed total: %d", result.ConsumedTotal)
	}
	if !bytes.Equal(result.RenegotiateToken, input) {
		t.Fatalf("unexpected renegotiate token: %q", string(result.RenegotiateToken))
	}
}

func certChainContextForTest(certs ...*windows.CertContext) *windows.CertChainContext {
	elements := make([]*windows.CertChainElement, 0, len(certs))
	for _, cert := range certs {
		elements = append(elements, &windows.CertChainElement{CertContext: cert})
	}
	simpleChain := &windows.CertSimpleChain{
		NumElements: uint32(len(elements)),
		Elements:    &elements[0],
	}
	chains := []*windows.CertSimpleChain{simpleChain}
	return &windows.CertChainContext{
		ChainCount: 1,
		Chains:     &chains[0],
	}
}

func certContextForTest(der, issuer, subject []byte) *windows.CertContext {
	certInfo := &windows.CertInfo{
		Issuer:  certNameBlobForTest(issuer),
		Subject: certNameBlobForTest(subject),
	}
	return &windows.CertContext{
		EncodedCert: &der[0],
		Length:      uint32(len(der)),
		CertInfo:    certInfo,
	}
}

func certNameBlobForTest(value []byte) windows.CertNameBlob {
	return windows.CertNameBlob{
		Size: uint32(len(value)),
		Data: (*byte)(unsafe.Pointer(&value[0])),
	}
}
