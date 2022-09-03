// Copyright 2020-2021 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package tls

// Delegated Credentials for TLS
// (https://tools.ietf.org/html/draft-ietf-tls-subcerts) is an IETF Internet
// draft and proposed TLS extension. If the client or server supports this
// extension, then the server or client may use a "delegated credential" as the
// signing key in the handshake. A delegated credential is a short lived
// public/secret key pair delegated to the peer by an entity trusted by the
// corresponding peer. This allows a reverse proxy to terminate a TLS connection
// on behalf of the entity. Credentials can't be revoked; in order to
// mitigate risk in case the reverse proxy is compromised, the credential is only
// valid for a short time (days, hours, or even minutes).

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/cryptobyte"
)

const (
	// In the absence of an application profile standard specifying otherwise,
	// the maximum validity period is set to 7 days.
	dcMaxTTLSeconds   = 60 * 60 * 24 * 7
	dcMaxTTL          = time.Duration(dcMaxTTLSeconds * time.Second)
	dcMaxPubLen       = (1 << 24) - 1 // Bytes
	dcMaxSignatureLen = (1 << 16) - 1 // Bytes
)

const (
	undefinedSignatureScheme SignatureScheme = 0x0000
)

var extensionDelegatedCredential = []int{1, 3, 6, 1, 4, 1, 44363, 44}

// isValidForDelegation returns true if a certificate can be used for Delegated
// Credentials.
func isValidForDelegation(cert *x509.Certificate) bool {
	// Check that the digitalSignature key usage is set.
	// The certificate must contains the digitalSignature KeyUsage.
	if (cert.KeyUsage & x509.KeyUsageDigitalSignature) == 0 {
		return false
	}

	// Check that the certificate has the DelegationUsage extension and that
	// it's marked as non-critical (See Section 4.2 of RFC5280).
	for _, extension := range cert.Extensions {
		if extension.Id.Equal(extensionDelegatedCredential) {
			if extension.Critical {
				return false
			}
			return true
		}
	}
	return false
}

// isExpired returns true if the credential has expired. The end of the validity
// interval is defined as the delegator certificate's notBefore field ('start')
// plus dc.cred.validTime seconds. This function simply checks that the current time
// ('now') is before the end of the validity interval.
func (dc *DelegatedCredential) isExpired(start, now time.Time) bool {
	end := start.Add(dc.cred.validTime)
	return !now.Before(end)
}

// invalidTTL returns true if the credential's validity period is longer than the
// maximum permitted. This is defined by the certificate's notBefore field
// ('start') plus the dc.validTime, minus the current time ('now').
func (dc *DelegatedCredential) invalidTTL(start, now time.Time) bool {
	return dc.cred.validTime > (now.Sub(start) + dcMaxTTL).Round(time.Second)
}

// credential stores the public components of a Delegated Credential.
type credential struct {
	// The amount of time for which the credential is valid. Specifically, the
	// the credential expires 'validTime' seconds after the 'notBefore' of the
	// delegation certificate. The delegator shall not issue Delegated
	// Credentials that are valid for more than 7 days from the current time.
	//
	// When this data structure is serialized, this value is converted to a
	// uint32 representing the duration in seconds.
	validTime time.Duration
	// The signature scheme associated with the credential public key.
	// This is expected to be the same as the CertificateVerify.algorithm
	// sent by the client or server.
	expCertVerfAlgo SignatureScheme
	// The credential's public key.
	publicKey crypto.PublicKey
}

// DelegatedCredential stores a Delegated Credential with the credential and its
// signature.
type DelegatedCredential struct {
	// The serialized form of the Delegated Credential.
	raw []byte

	// Cred stores the public components of a Delegated Credential.
	cred *credential

	// The signature scheme used to sign the Delegated Credential.
	algorithm SignatureScheme

	// The Credential's delegation: a signature that binds the credential to
	// the end-entity certificate's public key.
	signature []byte
}

// marshalPublicKeyInfo returns a DER encoded PublicKeyInfo
// from a Delegated Credential (as defined in the X.509 standard).
// The following key types are currently supported: *ecdsa.PublicKey
// and ed25519.PublicKey. Unsupported key types result in an error.
// rsa.PublicKey is not supported as defined by the draft.
func (cred *credential) marshalPublicKeyInfo() ([]byte, error) {
	switch cred.expCertVerfAlgo {
	case ECDSAWithP256AndSHA256,
		ECDSAWithP384AndSHA384,
		ECDSAWithP521AndSHA512,
		Ed25519:
		rawPub, err := x509.MarshalPKIXPublicKey(cred.publicKey)
		if err != nil {
			return nil, err
		}

		return rawPub, nil

	default:
		return nil, fmt.Errorf("tls: unsupported signature scheme: 0x%04x", cred.expCertVerfAlgo)
	}
}

// marshal encodes the credential struct of the Delegated Credential.
func (cred *credential) marshal() ([]byte, error) {
	var b cryptobyte.Builder

	b.AddUint32(uint32(cred.validTime / time.Second))
	b.AddUint16(uint16(cred.expCertVerfAlgo))

	// Encode the public key
	rawPub, err := cred.marshalPublicKeyInfo()
	if err != nil {
		return nil, err
	}
	// Assert that the public key encoding is no longer than 2^24-1 bytes.
	if len(rawPub) > dcMaxPubLen {
		return nil, errors.New("tls: public key length exceeds 2^24-1 limit")
	}

	b.AddUint24(uint32(len(rawPub)))
	b.AddBytes(rawPub)

	raw := b.BytesOrPanic()
	return raw, nil
}

// unmarshalCredential decodes serialized bytes and returns a credential, if possible.
func unmarshalCredential(raw []byte) (*credential, error) {
	if len(raw) < 10 {
		return nil, errors.New("tls: Delegated Credential is not valid: invalid length")
	}

	s := cryptobyte.String(raw)
	var t uint32
	if !s.ReadUint32(&t) {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}
	validTime := time.Duration(t) * time.Second

	var pubAlgo uint16
	if !s.ReadUint16(&pubAlgo) {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}
	algo := SignatureScheme(pubAlgo)

	var pubLen uint32
	s.ReadUint24(&pubLen)

	pubKey, err := x509.ParsePKIXPublicKey(s)
	if err != nil {
		return nil, err
	}

	return &credential{validTime, algo, pubKey}, nil
}

// getCredentialLen returns the number of bytes comprising the serialized
// credential struct inside the Delegated Credential.
func getCredentialLen(raw []byte) (int, error) {
	if len(raw) < 10 {
		return 0, errors.New("tls: Delegated Credential is not valid")
	}

	var read []byte
	s := cryptobyte.String(raw)
	s.ReadBytes(&read, 6)

	var pubLen uint32
	s.ReadUint24(&pubLen)
	if !(pubLen > 0) {
		return 0, errors.New("tls: Delegated Credential is not valid")
	}

	raw = raw[6:]
	if len(raw) < int(pubLen) {
		return 0, errors.New("tls: Delegated Credential is not valid")
	}

	return 9 + int(pubLen), nil
}

// getHash maps the SignatureScheme to its corresponding hash function.
func getHash(scheme SignatureScheme) crypto.Hash {
	switch scheme {
	case ECDSAWithP256AndSHA256:
		return crypto.SHA256
	case ECDSAWithP384AndSHA384:
		return crypto.SHA384
	case ECDSAWithP521AndSHA512:
		return crypto.SHA512
	case Ed25519:
		return directSigning
	case PKCS1WithSHA256, PSSWithSHA256:
		return crypto.SHA256
	case PSSWithSHA384:
		return crypto.SHA384
	case PSSWithSHA512:
		return crypto.SHA512
	default:
		return 0 // Unknown hash function
	}
}

// getECDSACurve maps the SignatureScheme to its corresponding ecdsa elliptic.Curve.
func getECDSACurve(scheme SignatureScheme) elliptic.Curve {
	switch scheme {
	case ECDSAWithP256AndSHA256:
		return elliptic.P256()
	case ECDSAWithP384AndSHA384:
		return elliptic.P384()
	case ECDSAWithP521AndSHA512:
		return elliptic.P521()
	default:
		return nil
	}
}

// prepareDelegationSignatureInput returns the message that the delegator is going to sign.
func prepareDelegationSignatureInput(hash crypto.Hash, cred *credential, dCert []byte, algo SignatureScheme, isClient bool) ([]byte, error) {
	header := make([]byte, 64)
	for i := range header {
		header[i] = 0x20
	}

	var context string
	if !isClient {
		context = "TLS, server delegated credentials\x00"
	} else {
		context = "TLS, client delegated credentials\x00"
	}

	rawCred, err := cred.marshal()
	if err != nil {
		return nil, err
	}

	var rawAlgo [2]byte
	binary.BigEndian.PutUint16(rawAlgo[:], uint16(algo))

	if hash == directSigning {
		b := &bytes.Buffer{}
		b.Write(header)
		io.WriteString(b, context)
		b.Write(dCert)
		b.Write(rawCred)
		b.Write(rawAlgo[:])
		return b.Bytes(), nil
	}

	h := hash.New()
	h.Write(header)
	io.WriteString(h, context)
	h.Write(dCert)
	h.Write(rawCred)
	h.Write(rawAlgo[:])
	return h.Sum(nil), nil
}

// Extract the algorithm used to sign the Delegated Credential from the
// end-entity (leaf) certificate.
func getSignatureAlgorithm(cert *Certificate) (SignatureScheme, error) {
	switch sk := cert.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		pk := sk.Public().(*ecdsa.PublicKey)
		curveName := pk.Curve.Params().Name
		certAlg := cert.Leaf.PublicKeyAlgorithm
		if certAlg == x509.ECDSA && curveName == "P-256" {
			return ECDSAWithP256AndSHA256, nil
		} else if certAlg == x509.ECDSA && curveName == "P-384" {
			return ECDSAWithP384AndSHA384, nil
		} else if certAlg == x509.ECDSA && curveName == "P-521" {
			return ECDSAWithP521AndSHA512, nil
		} else {
			return undefinedSignatureScheme, fmt.Errorf("using curve %s for %s is not supported", curveName, cert.Leaf.SignatureAlgorithm)
		}
	case ed25519.PrivateKey:
		return Ed25519, nil
	case *rsa.PrivateKey:
		// If the certificate has the RSAEncryption OID there are a number of valid signature schemes that may sign the DC.
		// In the absence of better information, we make a reasonable choice.
		return PSSWithSHA256, nil
	default:
		return undefinedSignatureScheme, fmt.Errorf("tls: unsupported algorithm for signing Delegated Credential")
	}
}

// NewDelegatedCredential creates a new Delegated Credential using 'cert' for
// delegation, depending if the caller is the client or the server (defined by
// 'isClient'). It generates a public/private key pair for the provided signature
// algorithm ('pubAlgo') and it defines a validity interval (defined
// by 'cert.Leaf.notBefore' and 'validTime'). It signs the Delegated Credential
// using 'cert.PrivateKey'.
func NewDelegatedCredential(cert *Certificate, pubAlgo SignatureScheme, validTime time.Duration, isClient bool) (*DelegatedCredential, crypto.PrivateKey, error) {
	// The granularity of DC validity is seconds.
	validTime = validTime.Round(time.Second)

	// Parse the leaf certificate if needed.
	var err error
	if cert.Leaf == nil {
		if len(cert.Certificate[0]) == 0 {
			return nil, nil, errors.New("tls: missing leaf certificate for Delegated Credential")
		}
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, nil, err
		}
	}

	// Check that the leaf certificate can be used for delegation.
	if !isValidForDelegation(cert.Leaf) {
		return nil, nil, errors.New("tls: certificate not authorized for delegation")
	}

	sigAlgo, err := getSignatureAlgorithm(cert)
	if err != nil {
		return nil, nil, err
	}

	// Generate the Delegated Credential key pair based on the provided scheme
	var privK crypto.PrivateKey
	var pubK crypto.PublicKey
	switch pubAlgo {
	case ECDSAWithP256AndSHA256,
		ECDSAWithP384AndSHA384,
		ECDSAWithP521AndSHA512:
		privK, err = ecdsa.GenerateKey(getECDSACurve(pubAlgo), rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		pubK = privK.(*ecdsa.PrivateKey).Public()
	case Ed25519:
		pubK, privK, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("tls: unsupported algorithm for Delegated Credential: %s", pubAlgo)
	}

	// Prepare the credential for signing
	hash := getHash(sigAlgo)
	credential := &credential{validTime, pubAlgo, pubK}
	values, err := prepareDelegationSignatureInput(hash, credential, cert.Leaf.Raw, sigAlgo, isClient)
	if err != nil {
		return nil, nil, err
	}

	var sig []byte
	switch sk := cert.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		opts := crypto.SignerOpts(hash)
		sig, err = sk.Sign(rand.Reader, values, opts)
		if err != nil {
			return nil, nil, err
		}
	case ed25519.PrivateKey:
		opts := crypto.SignerOpts(hash)
		sig, err = sk.Sign(rand.Reader, values, opts)
		if err != nil {
			return nil, nil, err
		}
	case *rsa.PrivateKey:
		opts := &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		}
		sig, err = rsa.SignPSS(rand.Reader, sk, hash, values, opts)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("tls: unsupported key type for Delegated Credential")
	}

	if len(sig) > dcMaxSignatureLen {
		return nil, nil, errors.New("tls: unable to create a Delegated Credential")
	}

	return &DelegatedCredential{
		cred:      credential,
		algorithm: sigAlgo,
		signature: sig,
	}, privK, nil
}

// Validate validates the Delegated Credential by checking that the signature is
// valid, that it hasn't expired, and that the TTL is valid. It also checks that
// certificate can be used for delegation.
func (dc *DelegatedCredential) Validate(cert *x509.Certificate, isClient bool, now time.Time, certVerifyMsg *certificateVerifyMsg) bool {
	if dc.isExpired(cert.NotBefore, now) {
		return false
	}

	if dc.invalidTTL(cert.NotBefore, now) {
		return false
	}

	if dc.cred.expCertVerfAlgo != certVerifyMsg.signatureAlgorithm {
		return false
	}

	if !isValidForDelegation(cert) {
		return false
	}

	hash := getHash(dc.algorithm)
	in, err := prepareDelegationSignatureInput(hash, dc.cred, cert.Raw, dc.algorithm, isClient)
	if err != nil {
		return false
	}

	switch dc.algorithm {
	case ECDSAWithP256AndSHA256,
		ECDSAWithP384AndSHA384,
		ECDSAWithP521AndSHA512:
		pk, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return false
		}

		return ecdsa.VerifyASN1(pk, in, dc.signature)
	case Ed25519:
		pk, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok {
			return false
		}

		return ed25519.Verify(pk, in, dc.signature)
	case PSSWithSHA256,
		PSSWithSHA384,
		PSSWithSHA512:
		pk, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return false
		}
		hash := getHash(dc.algorithm)
		return rsa.VerifyPSS(pk, hash, in, dc.signature, nil) == nil
	default:
		return false
	}
}

// Marshal encodes a DelegatedCredential structure. It also sets dc.Raw to that
// encoding.
func (dc *DelegatedCredential) Marshal() ([]byte, error) {
	if len(dc.signature) > dcMaxSignatureLen {
		return nil, errors.New("tls: delegated credential is not valid")
	}
	if len(dc.signature) == 0 {
		return nil, errors.New("tls: delegated credential has no signature")
	}

	raw, err := dc.cred.marshal()
	if err != nil {
		return nil, err
	}

	var b cryptobyte.Builder
	b.AddBytes(raw)
	b.AddUint16(uint16(dc.algorithm))
	b.AddUint16(uint16(len(dc.signature)))
	b.AddBytes(dc.signature)

	dc.raw = b.BytesOrPanic()
	return dc.raw, nil
}

// UnmarshalDelegatedCredential decodes a DelegatedCredential structure.
func UnmarshalDelegatedCredential(raw []byte) (*DelegatedCredential, error) {
	rawCredentialLen, err := getCredentialLen(raw)
	if err != nil {
		return nil, err
	}

	credential, err := unmarshalCredential(raw[:rawCredentialLen])
	if err != nil {
		return nil, err
	}

	raw = raw[rawCredentialLen:]
	if len(raw) < 4 {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}

	s := cryptobyte.String(raw)

	var algo uint16
	if !s.ReadUint16(&algo) {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}

	var rawSignatureLen uint16
	if !s.ReadUint16(&rawSignatureLen) {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}

	var sig []byte
	if !s.ReadBytes(&sig, int(rawSignatureLen)) {
		return nil, errors.New("tls: Delegated Credential is not valid")
	}

	return &DelegatedCredential{
		cred:      credential,
		algorithm: SignatureScheme(algo),
		signature: sig,
	}, nil
}
