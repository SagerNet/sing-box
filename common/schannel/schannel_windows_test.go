//go:build windows

package schannel

import (
	"bytes"
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
