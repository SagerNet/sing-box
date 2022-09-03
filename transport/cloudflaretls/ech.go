// Copyright 2020 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package tls

import (
	"errors"
	"fmt"
	"io"

	"github.com/cloudflare/circl/hpke"
	"golang.org/x/crypto/cryptobyte"
)

const (
	// Constants for TLS operations
	echAcceptConfLabel    = "ech accept confirmation"
	echAcceptConfHRRLabel = "hrr ech accept confirmation"

	// Constants for HPKE operations
	echHpkeInfoSetup = "tls ech"

	// When sent in the ClientHello, the first byte of the payload of the ECH
	// extension indicates whether the message is the ClientHelloOuter or
	// ClientHelloInner.
	echClientHelloOuterVariant uint8 = 0
	echClientHelloInnerVariant uint8 = 1
)

var zeros = [8]byte{}

// echOfferOrGrease is called by the client after generating its ClientHello
// message to decide if it will offer or GREASE ECH. It does neither if ECH is
// disabled. Returns a pair of ClientHello messages, hello and helloInner. If
// offering ECH, these are the ClienthelloOuter and ClientHelloInner
// respectively. Otherwise, hello is the ClientHello and helloInner == nil.
//
// TODO(cjpatton): "[When offering ECH, the client] MUST NOT offer to resume any
// session for TLS 1.2 and below [in ClientHelloInner]."
func (c *Conn) echOfferOrGrease(helloBase *clientHelloMsg) (hello, helloInner *clientHelloMsg, err error) {
	config := c.config

	if !config.ECHEnabled || testingECHTriggerBypassBeforeHRR {
		// Bypass ECH.
		return helloBase, nil, nil
	}

	// Choose the ECHConfig to use for this connection. If none is available, or
	// if we're not offering TLS 1.3 or above, then GREASE.
	echConfig := config.echSelectConfig()
	if echConfig == nil || config.maxSupportedVersion(roleClient) < VersionTLS13 {
		var err error

		// Generate a dummy ClientECH.
		helloBase.ech, err = echGenerateGreaseExt(config.rand())
		if err != nil {
			return nil, nil, fmt.Errorf("tls: ech: failed to generate grease ECH: %s", err)
		}

		// GREASE ECH.
		c.ech.offered = false
		c.ech.greased = true
		helloBase.raw = nil
		return helloBase, nil, nil
	}

	// Store the ECH config parameters that are needed later.
	c.ech.configId = echConfig.configId
	c.ech.maxNameLen = int(echConfig.maxNameLen)

	// Generate the HPKE context. Store it in case of HRR.
	var enc []byte
	enc, c.ech.sealer, err = echConfig.setupSealer(config.rand())
	if err != nil {
		return nil, nil, fmt.Errorf("tls: ech: %s", err)
	}

	// ClientHelloInner is constructed from the base ClientHello. The payload of
	// the "encrypted_client_hello" extension is a single 1 byte indicating that
	// this is the ClientHelloInner.
	helloInner = helloBase
	helloInner.ech = []byte{echClientHelloInnerVariant}

	// Ensure that only TLS 1.3 and above are offered in the inner handshake.
	if v := helloInner.supportedVersions; len(v) == 0 || v[len(v)-1] < VersionTLS13 {
		return nil, nil, errors.New("tls: ech: only TLS 1.3 is allowed in ClientHelloInner")
	}

	// ClientHelloOuter is constructed by generating a fresh ClientHello and
	// copying "session_id" from ClientHelloInner, setting "server_name" to the
	// client-facing server, and adding the "encrypted_client_hello" extension.
	//
	// In addition, we discard the "key_share" and instead use the one from
	// ClientHelloInner.
	hello, _, err = c.makeClientHello(config.MinVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("tls: ech: %s", err)
	}
	hello.sessionId = helloBase.sessionId
	hello.serverName = hostnameInSNI(string(echConfig.rawPublicName))
	if err := c.echUpdateClientHelloOuter(hello, helloInner, enc); err != nil {
		return nil, nil, err
	}

	// Offer ECH.
	c.ech.offered = true
	helloInner.raw = nil
	hello.raw = nil
	return hello, helloInner, nil
}

// echUpdateClientHelloOuter is called by the client to construct the payload of
// the ECH extension in the outer handshake.
func (c *Conn) echUpdateClientHelloOuter(hello, helloInner *clientHelloMsg, enc []byte) error {
	var (
		ech echClientOuter
		err error
	)

	// Copy all compressed extensions from ClientHelloInner into
	// ClientHelloOuter.
	for _, ext := range echOuterExtensions() {
		echCopyExtensionFromClientHelloInner(hello, helloInner, ext)
	}

	// Always copy the "key_shares" extension from ClientHelloInner, regardless
	// of whether it gets compressed.
	hello.keyShares = helloInner.keyShares

	_, kdf, aead := c.ech.sealer.Suite().Params()
	ech.handle.suite.kdfId = uint16(kdf)
	ech.handle.suite.aeadId = uint16(aead)
	ech.handle.configId = c.ech.configId
	ech.handle.enc = enc

	// EncodedClientHelloInner
	helloInner.raw = nil
	encodedHelloInner := echEncodeClientHelloInner(
		helloInner.marshal(),
		len(helloInner.serverName),
		c.ech.maxNameLen)
	if encodedHelloInner == nil {
		return errors.New("tls: ech: encoding of EncodedClientHelloInner failed")
	}

	// ClientHelloOuterAAD
	hello.raw = nil
	hello.ech = ech.marshal()
	helloOuterAad := echEncodeClientHelloOuterAAD(hello.marshal(),
		aead.CipherLen(uint(len(encodedHelloInner))))
	if helloOuterAad == nil {
		return errors.New("tls: ech: encoding of ClientHelloOuterAAD failed")
	}

	ech.payload, err = c.ech.sealer.Seal(encodedHelloInner, helloOuterAad)
	if err != nil {
		return fmt.Errorf("tls: ech: seal failed: %s", err)
	}
	if testingECHTriggerPayloadDecryptError {
		ech.payload[0] ^= 0xff // Inauthentic ciphertext
	}
	ech.raw = nil
	hello.ech = ech.marshal()

	helloInner.raw = nil
	hello.raw = nil
	return nil
}

// echAcceptOrReject is called by the client-facing server to determine whether
// ECH was offered by the client, and if so, whether to accept or reject. The
// return value is the ClientHello that will be used for the connection.
//
// This function is called prior to processing the ClientHello. In case of
// HelloRetryRequest, it is also called before processing the second
// ClientHello. This is indicated by the afterHRR flag.
func (c *Conn) echAcceptOrReject(hello *clientHelloMsg, afterHRR bool) (*clientHelloMsg, error) {
	config := c.config
	p := config.ServerECHProvider

	if !config.echCanAccept() {
		// Bypass ECH.
		return hello, nil
	}

	if len(hello.ech) > 0 { // The ECH extension is present
		switch hello.ech[0] {
		case echClientHelloInnerVariant: // inner handshake
			if len(hello.ech) > 1 {
				c.sendAlert(alertIllegalParameter)
				return nil, errors.New("ech: inner handshake has non-empty payload")
			}

			// Continue as the backend server.
			return hello, nil
		case echClientHelloOuterVariant: // outer handshake
		default:
			c.sendAlert(alertIllegalParameter)
			return nil, errors.New("ech: inner handshake has non-empty payload")
		}
	} else {
		if c.ech.offered {
			// This occurs if the server accepted prior to HRR, but the client
			// failed to send the ECH extension in the second ClientHelloOuter. This
			// would cause ClientHelloOuter to be used after ClientHelloInner, which
			// is illegal.
			c.sendAlert(alertMissingExtension)
			return nil, errors.New("ech: hrr: bypass after offer")
		}

		// Bypass ECH.
		return hello, nil
	}

	if afterHRR && !c.ech.offered && !c.ech.greased {
		// The client bypassed ECH prior to HRR, but not after. This could
		// cause ClientHelloInner to be used after ClientHelloOuter, which is
		// illegal.
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("ech: hrr: offer or grease after bypass")
	}

	// Parse ClientECH.
	ech, err := echUnmarshalClientOuter(hello.ech)
	if err != nil {
		c.sendAlert(alertIllegalParameter)
		return nil, fmt.Errorf("ech: failed to parse extension: %s", err)
	}

	// Make sure that the HPKE suite and config id don't change across HRR and
	// that the encapsulated key is not present after HRR.
	if afterHRR && c.ech.offered {
		_, kdf, aead := c.ech.opener.Suite().Params()
		if ech.handle.suite.kdfId != uint16(kdf) ||
			ech.handle.suite.aeadId != uint16(aead) ||
			ech.handle.configId != c.ech.configId ||
			len(ech.handle.enc) > 0 {
			c.sendAlert(alertIllegalParameter)
			return nil, errors.New("ech: hrr: illegal handle in second hello")
		}
	}

	// Store the config id in case of HRR.
	c.ech.configId = ech.handle.configId

	// Ask the ECH provider for the HPKE context.
	if c.ech.opener == nil {
		res := p.GetDecryptionContext(ech.handle.marshal(), extensionECH)

		// Compute retry configurations, skipping those indicating an
		// unsupported version.
		if len(res.RetryConfigs) > 0 {
			configs, err := UnmarshalECHConfigs(res.RetryConfigs) // skips unrecognized versions
			if err != nil {
				c.sendAlert(alertInternalError)
				return nil, fmt.Errorf("ech: %s", err)
			}

			if len(configs) > 0 {
				c.ech.retryConfigs, err = echMarshalConfigs(configs)
				if err != nil {
					c.sendAlert(alertInternalError)
					return nil, fmt.Errorf("ech: %s", err)
				}
			}

			// Check if the outer SNI matches the public name of any ECH config
			// advertised by the client-facing server. As of
			// draft-ietf-tls-esni-10, the client is required to use the ECH
			// config's public name as the outer SNI. Although there's no real
			// reason for the server to enforce this, it's worth noting it when
			// it happens.
			pubNameMatches := false
			for _, config := range configs {
				if hello.serverName == string(config.rawPublicName) {
					pubNameMatches = true
				}
			}
			if !pubNameMatches {
				c.handleCFEvent(CFEventECHPublicNameMismatch{})
			}
		}

		switch res.Status {
		case ECHProviderSuccess:
			c.ech.opener, err = hpke.UnmarshalOpener(res.Context)
			if err != nil {
				c.sendAlert(alertInternalError)
				return nil, fmt.Errorf("ech: %s", err)
			}
		case ECHProviderReject:
			// Reject ECH. We do not know at this point whether the client
			// intended to offer or grease ECH, so we presume grease until the
			// client indicates rejection by sending an "ech_required" alert.
			c.ech.greased = true
			return hello, nil
		case ECHProviderAbort:
			c.sendAlert(alert(res.Alert))
			return nil, fmt.Errorf("ech: provider aborted: %s", res.Error)
		default:
			c.sendAlert(alertInternalError)
			return nil, errors.New("ech: unexpected provider status")
		}
	}

	// ClientHelloOuterAAD
	rawHelloOuterAad := echEncodeClientHelloOuterAAD(hello.marshal(), uint(len(ech.payload)))
	if rawHelloOuterAad == nil {
		// This occurs if the ClientHelloOuter is malformed. This values was
		// already parsed into `hello`, so this should not happen.
		c.sendAlert(alertInternalError)
		return nil, fmt.Errorf("ech: failed to encode ClientHelloOuterAAD")
	}

	// EncodedClientHelloInner
	rawEncodedHelloInner, err := c.ech.opener.Open(ech.payload, rawHelloOuterAad)
	if err != nil {
		if afterHRR && c.ech.accepted {
			// Don't reject after accept, as this would result in processing the
			// ClientHelloOuter after processing the ClientHelloInner.
			c.sendAlert(alertDecryptError)
			return nil, fmt.Errorf("ech: hrr: reject after accept: %s", err)
		}

		// Reject ECH. We do not know at this point whether the client
		// intended to offer or grease ECH, so we presume grease until the
		// client indicates rejection by sending an "ech_required" alert.
		c.ech.greased = true
		return hello, nil
	}

	// ClientHelloInner
	rawHelloInner := echDecodeClientHelloInner(rawEncodedHelloInner, hello.marshal(), hello.sessionId)
	if rawHelloInner == nil {
		c.sendAlert(alertIllegalParameter)
		return nil, fmt.Errorf("ech: failed to decode EncodedClientHelloInner")
	}
	helloInner := new(clientHelloMsg)
	if !helloInner.unmarshal(rawHelloInner) {
		c.sendAlert(alertIllegalParameter)
		return nil, fmt.Errorf("ech: failed to parse ClientHelloInner")
	}

	// Check for a well-formed ECH extension.
	if len(helloInner.ech) != 1 ||
		helloInner.ech[0] != echClientHelloInnerVariant {
		c.sendAlert(alertIllegalParameter)
		return nil, fmt.Errorf("ech: ClientHelloInner does not have a well-formed ECH extension")
	}

	// Check that the client did not offer TLS 1.2 or below in the inner
	// handshake.
	helloInnerSupportsTLS12OrBelow := len(helloInner.supportedVersions) == 0
	for _, v := range helloInner.supportedVersions {
		if v < VersionTLS13 {
			helloInnerSupportsTLS12OrBelow = true
		}
	}
	if helloInnerSupportsTLS12OrBelow {
		c.sendAlert(alertIllegalParameter)
		return nil, errors.New("ech: ClientHelloInner offers TLS 1.2 or below")
	}

	// Accept ECH.
	c.ech.offered = true
	c.ech.accepted = true
	return helloInner, nil
}

// echClientOuter represents a ClientECH structure, the payload of the client's
// "encrypted_client_hello" extension that appears in the outer handshake.
type echClientOuter struct {
	raw []byte

	// Parsed from raw
	handle  echContextHandle
	payload []byte
}

// echUnmarshalClientOuter parses a ClientECH structure. The caller provides the
// ECH version indicated by the client.
func echUnmarshalClientOuter(raw []byte) (*echClientOuter, error) {
	s := cryptobyte.String(raw)
	ech := new(echClientOuter)
	ech.raw = raw

	// Make sure this is the outer handshake.
	var variant uint8
	if !s.ReadUint8(&variant) {
		return nil, fmt.Errorf("error parsing ClientECH.type")
	}
	if variant != echClientHelloOuterVariant {
		return nil, fmt.Errorf("unexpected ClientECH.type (want outer (0))")
	}

	// Parse the context handle.
	if !echReadContextHandle(&s, &ech.handle) {
		return nil, fmt.Errorf("error parsing context handle")
	}
	endOfContextHandle := len(raw) - len(s)
	ech.handle.raw = raw[1:endOfContextHandle]

	// Parse the payload.
	var t cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&t) ||
		!t.ReadBytes(&ech.payload, len(t)) || !s.Empty() {
		return nil, fmt.Errorf("error parsing payload")
	}

	return ech, nil
}

func (ech *echClientOuter) marshal() []byte {
	if ech.raw != nil {
		return ech.raw
	}
	var b cryptobyte.Builder
	b.AddUint8(echClientHelloOuterVariant)
	b.AddBytes(ech.handle.marshal())
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(ech.payload)
	})
	return b.BytesOrPanic()
}

// echContextHandle represents the prefix of a ClientECH structure used by
// the server to compute the HPKE context.
type echContextHandle struct {
	raw []byte

	// Parsed from raw
	suite    hpkeSymmetricCipherSuite
	configId uint8
	enc      []byte
}

func (handle *echContextHandle) marshal() []byte {
	if handle.raw != nil {
		return handle.raw
	}
	var b cryptobyte.Builder
	b.AddUint16(handle.suite.kdfId)
	b.AddUint16(handle.suite.aeadId)
	b.AddUint8(handle.configId)
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(handle.enc)
	})
	return b.BytesOrPanic()
}

func echReadContextHandle(s *cryptobyte.String, handle *echContextHandle) bool {
	var t cryptobyte.String
	if !s.ReadUint16(&handle.suite.kdfId) || // cipher_suite.kdf_id
		!s.ReadUint16(&handle.suite.aeadId) || // cipher_suite.aead_id
		!s.ReadUint8(&handle.configId) || // config_id
		!s.ReadUint16LengthPrefixed(&t) || // enc
		!t.ReadBytes(&handle.enc, len(t)) {
		return false
	}
	return true
}

// hpkeSymmetricCipherSuite represents an ECH ciphersuite, a KDF/AEAD algorithm pair. This
// is different from an HPKE ciphersuite, which represents a KEM/KDF/AEAD
// triple.
type hpkeSymmetricCipherSuite struct {
	kdfId, aeadId uint16
}

// Generates a grease ECH extension using a hard-coded KEM public key.
func echGenerateGreaseExt(rand io.Reader) ([]byte, error) {
	var err error
	dummyX25519PublicKey := []byte{
		143, 38, 37, 36, 12, 6, 229, 30, 140, 27, 167, 73, 26, 100, 203, 107, 216,
		81, 163, 222, 52, 211, 54, 210, 46, 37, 78, 216, 157, 97, 241, 244,
	}
	dummyEncodedHelloInnerLen := 100 // TODO(cjpatton): Compute this correctly.
	kem, kdf, aead := defaultHPKESuite.Params()

	pk, err := kem.Scheme().UnmarshalBinaryPublicKey(dummyX25519PublicKey)
	if err != nil {
		return nil, fmt.Errorf("tls: grease ech: failed to parse dummy public key: %s", err)
	}
	sender, err := defaultHPKESuite.NewSender(pk, nil)
	if err != nil {
		return nil, fmt.Errorf("tls: grease ech: failed to create sender: %s", err)
	}

	var ech echClientOuter
	ech.handle.suite.kdfId = uint16(kdf)
	ech.handle.suite.aeadId = uint16(aead)
	randomByte := make([]byte, 1)
	_, err = io.ReadFull(rand, randomByte)
	if err != nil {
		return nil, fmt.Errorf("tls: grease ech: %s", err)
	}
	ech.handle.configId = randomByte[0]
	ech.handle.enc, _, err = sender.Setup(rand)
	if err != nil {
		return nil, fmt.Errorf("tls: grease ech: %s", err)
	}
	ech.payload = make([]byte,
		int(aead.CipherLen(uint(dummyEncodedHelloInnerLen))))
	if _, err = io.ReadFull(rand, ech.payload); err != nil {
		return nil, fmt.Errorf("tls: grease ech: %s", err)
	}
	return ech.marshal(), nil
}

// echEncodeClientHelloInner interprets innerData as a ClientHelloInner message
// and transforms it into an EncodedClientHelloInner. Returns nil if parsing
// innerData fails.
func echEncodeClientHelloInner(innerData []byte, serverNameLen, maxNameLen int) []byte {
	var (
		errIllegalParameter      = errors.New("illegal parameter")
		outerExtensions          = echOuterExtensions()
		msgType                  uint8
		legacyVersion            uint16
		random                   []byte
		legacySessionId          cryptobyte.String
		cipherSuites             cryptobyte.String
		legacyCompressionMethods cryptobyte.String
		extensions               cryptobyte.String
		s                        cryptobyte.String
		b                        cryptobyte.Builder
	)

	u := cryptobyte.String(innerData)
	if !u.ReadUint8(&msgType) ||
		!u.ReadUint24LengthPrefixed(&s) || !u.Empty() {
		return nil
	}

	if !s.ReadUint16(&legacyVersion) ||
		!s.ReadBytes(&random, 32) ||
		!s.ReadUint8LengthPrefixed(&legacySessionId) ||
		!s.ReadUint16LengthPrefixed(&cipherSuites) ||
		!s.ReadUint8LengthPrefixed(&legacyCompressionMethods) {
		return nil
	}

	if s.Empty() {
		// Extensions field must be present in TLS 1.3.
		return nil
	}

	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return nil
	}

	b.AddUint16(legacyVersion)
	b.AddBytes(random)
	b.AddUint8(0) // 0-length legacy_session_id
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(cipherSuites)
	})
	b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(legacyCompressionMethods)
	})
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		if testingECHOuterExtIncorrectOrder {
			// Replace outer extensions with "outer_extension" extension, but in
			// the incorrect order.
			echAddOuterExtensions(b, outerExtensions)
		}

		for !extensions.Empty() {
			var ext uint16
			var extData cryptobyte.String
			if !extensions.ReadUint16(&ext) ||
				!extensions.ReadUint16LengthPrefixed(&extData) {
				panic(cryptobyte.BuildError{Err: errIllegalParameter})
			}

			if len(outerExtensions) > 0 && ext == outerExtensions[0] {
				if !testingECHOuterExtIncorrectOrder {
					// Replace outer extensions with "outer_extension" extension.
					echAddOuterExtensions(b, outerExtensions)
				}

				// Consume the remaining outer extensions.
				for _, outerExt := range outerExtensions[1:] {
					if !extensions.ReadUint16(&ext) ||
						!extensions.ReadUint16LengthPrefixed(&extData) {
						panic(cryptobyte.BuildError{Err: errIllegalParameter})
					}
					if ext != outerExt {
						panic("internal error: malformed ClientHelloInner")
					}
				}

			} else {
				b.AddUint16(ext)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					b.AddBytes(extData)
				})
			}
		}
	})

	encodedData, err := b.Bytes()
	if err == errIllegalParameter {
		return nil // Input malformed
	} else if err != nil {
		panic(err) // Host encountered internal error
	}

	// Add padding.
	paddingLen := 0
	if serverNameLen > 0 {
		// draft-ietf-tls-esni-13, Section 6.1.3:
		//
		// If the ClientHelloInner contained a "server_name" extension with a
		// name of length D, add max(0, L - D) bytes of padding.
		if n := maxNameLen - serverNameLen; n > 0 {
			paddingLen += n
		}
	} else {
		// draft-ietf-tls-esni-13, Section 6.1.3:
		//
		// If the ClientHelloInner did not contain a "server_name" extension
		// (e.g., if the client is connecting to an IP address), add L + 9 bytes
		// of padding.  This is the length of a "server_name" extension with an
		// L-byte name.
		const sniPaddingLen = 9
		paddingLen += sniPaddingLen + maxNameLen
	}
	paddingLen = 31 - ((len(encodedData) + paddingLen - 1) % 32)
	for i := 0; i < paddingLen; i++ {
		encodedData = append(encodedData, 0)
	}

	return encodedData
}

func echAddOuterExtensions(b *cryptobyte.Builder, outerExtensions []uint16) {
	b.AddUint16(extensionECHOuterExtensions)
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			for _, outerExt := range outerExtensions {
				b.AddUint16(outerExt)
			}
			if testingECHOuterExtIllegal {
				// This is not allowed.
				b.AddUint16(extensionECH)
			}
		})
	})
}

// echDecodeClientHelloInner interprets encodedData as an EncodedClientHelloInner
// message and substitutes the "outer_extension" extension with extensions from
// outerData, interpreted as the ClientHelloOuter message. Returns nil if
// parsing encodedData fails.
func echDecodeClientHelloInner(encodedData, outerData, outerSessionId []byte) []byte {
	var (
		errIllegalParameter      = errors.New("illegal parameter")
		legacyVersion            uint16
		random                   []byte
		legacySessionId          cryptobyte.String
		cipherSuites             cryptobyte.String
		legacyCompressionMethods cryptobyte.String
		extensions               cryptobyte.String
		b                        cryptobyte.Builder
	)

	s := cryptobyte.String(encodedData)
	if !s.ReadUint16(&legacyVersion) ||
		!s.ReadBytes(&random, 32) ||
		!s.ReadUint8LengthPrefixed(&legacySessionId) ||
		!s.ReadUint16LengthPrefixed(&cipherSuites) ||
		!s.ReadUint8LengthPrefixed(&legacyCompressionMethods) {
		return nil
	}

	if len(legacySessionId) > 0 {
		return nil
	}

	if s.Empty() {
		// Extensions field must be present in TLS 1.3.
		return nil
	}

	if !s.ReadUint16LengthPrefixed(&extensions) {
		return nil
	}

	b.AddUint8(typeClientHello)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint16(legacyVersion)
		b.AddBytes(random)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(outerSessionId) // ClientHelloOuter.legacy_session_id
		})
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(cipherSuites)
		})
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(legacyCompressionMethods)
		})
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			var handledOuterExtensions bool
			for !extensions.Empty() {
				var ext uint16
				var extData cryptobyte.String
				if !extensions.ReadUint16(&ext) ||
					!extensions.ReadUint16LengthPrefixed(&extData) {
					panic(cryptobyte.BuildError{Err: errIllegalParameter})
				}

				if ext == extensionECHOuterExtensions {
					if handledOuterExtensions {
						// It is an error to send any extension more than once in a
						// single message.
						panic(cryptobyte.BuildError{Err: errIllegalParameter})
					}
					handledOuterExtensions = true

					// Read the referenced outer extensions.
					referencedExts := make([]uint16, 0, 10)
					var outerExtData cryptobyte.String
					if !extData.ReadUint8LengthPrefixed(&outerExtData) ||
						len(outerExtData)%2 != 0 ||
						!extData.Empty() {
						panic(cryptobyte.BuildError{Err: errIllegalParameter})
					}
					for !outerExtData.Empty() {
						if !outerExtData.ReadUint16(&ext) ||
							ext == extensionECH {
							panic(cryptobyte.BuildError{Err: errIllegalParameter})
						}
						referencedExts = append(referencedExts, ext)
					}

					// Add the outer extensions from the ClientHelloOuter into the
					// ClientHelloInner.
					outerCt := 0
					r := processClientHelloExtensions(outerData, func(ext uint16, extData cryptobyte.String) bool {
						if outerCt < len(referencedExts) && ext == referencedExts[outerCt] {
							outerCt++
							b.AddUint16(ext)
							b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
								b.AddBytes(extData)
							})
						}
						return true
					})

					// Ensure that all outer extensions have been incorporated
					// exactly once, and in the correct order.
					if !r || outerCt != len(referencedExts) {
						panic(cryptobyte.BuildError{Err: errIllegalParameter})
					}
				} else {
					b.AddUint16(ext)
					b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
						b.AddBytes(extData)
					})
				}
			}
		})
	})

	innerData, err := b.Bytes()
	if err == errIllegalParameter {
		return nil // Input malformed
	} else if err != nil {
		panic(err) // Host encountered internal error
	}

	// Read the padding.
	for !s.Empty() {
		var zero uint8
		if !s.ReadUint8(&zero) || zero != 0 {
			return nil
		}
	}

	return innerData
}

// echEncodeClientHelloOuterAAD interprets outerData as ClientHelloOuter and
// constructs a ClientHelloOuterAAD. The output doesn't have the 4-byte prefix
// that indicates the handshake message type and its length.
func echEncodeClientHelloOuterAAD(outerData []byte, payloadLen uint) []byte {
	var (
		errIllegalParameter      = errors.New("illegal parameter")
		msgType                  uint8
		legacyVersion            uint16
		random                   []byte
		legacySessionId          cryptobyte.String
		cipherSuites             cryptobyte.String
		legacyCompressionMethods cryptobyte.String
		extensions               cryptobyte.String
		s                        cryptobyte.String
		b                        cryptobyte.Builder
	)

	u := cryptobyte.String(outerData)
	if !u.ReadUint8(&msgType) ||
		!u.ReadUint24LengthPrefixed(&s) || !u.Empty() {
		return nil
	}

	if !s.ReadUint16(&legacyVersion) ||
		!s.ReadBytes(&random, 32) ||
		!s.ReadUint8LengthPrefixed(&legacySessionId) ||
		!s.ReadUint16LengthPrefixed(&cipherSuites) ||
		!s.ReadUint8LengthPrefixed(&legacyCompressionMethods) {
		return nil
	}

	if s.Empty() {
		// Extensions field must be present in TLS 1.3.
		return nil
	}

	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return nil
	}

	b.AddUint16(legacyVersion)
	b.AddBytes(random)
	b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(legacySessionId)
	})
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(cipherSuites)
	})
	b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(legacyCompressionMethods)
	})
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		for !extensions.Empty() {
			var ext uint16
			var extData cryptobyte.String
			if !extensions.ReadUint16(&ext) ||
				!extensions.ReadUint16LengthPrefixed(&extData) {
				panic(cryptobyte.BuildError{Err: errIllegalParameter})
			}

			// If this is the ECH extension and the payload is the outer variant
			// of ClientECH, then replace the payloadLen 0 bytes.
			if ext == extensionECH {
				ech, err := echUnmarshalClientOuter(extData)
				if err != nil {
					panic(cryptobyte.BuildError{Err: errIllegalParameter})
				}
				ech.payload = make([]byte, payloadLen)
				ech.raw = nil
				extData = ech.marshal()
			}

			b.AddUint16(ext)
			b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
				b.AddBytes(extData)
			})
		}
	})

	outerAadData, err := b.Bytes()
	if err == errIllegalParameter {
		return nil // Input malformed
	} else if err != nil {
		panic(err) // Host encountered internal error
	}

	return outerAadData
}

// echEncodeAcceptConfHelloRetryRequest interprets data as a ServerHello message
// and replaces the payload of the ECH extension with 8 zero bytes. The output
// includes the 4-byte prefix that indicates the message type and its length.
func echEncodeAcceptConfHelloRetryRequest(data []byte) []byte {
	var (
		errIllegalParameter = errors.New("illegal parameter")
		vers                uint16
		random              []byte
		sessionId           []byte
		cipherSuite         uint16
		compressionMethod   uint8
		s                   cryptobyte.String
		b                   cryptobyte.Builder
	)

	s = cryptobyte.String(data)
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint16(&vers) || !s.ReadBytes(&random, 32) ||
		!readUint8LengthPrefixed(&s, &sessionId) ||
		!s.ReadUint16(&cipherSuite) ||
		!s.ReadUint8(&compressionMethod) {
		return nil
	}

	if s.Empty() {
		// ServerHello is optionally followed by extension data
		return nil
	}

	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return nil
	}

	b.AddUint8(typeServerHello)
	b.AddUint24LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddUint16(vers)
		b.AddBytes(random)
		b.AddUint8LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes(sessionId)
		})
		b.AddUint16(cipherSuite)
		b.AddUint8(compressionMethod)
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			for !extensions.Empty() {
				var extension uint16
				var extData cryptobyte.String
				if !extensions.ReadUint16(&extension) ||
					!extensions.ReadUint16LengthPrefixed(&extData) {
					panic(cryptobyte.BuildError{Err: errIllegalParameter})
				}

				b.AddUint16(extension)
				b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
					if extension == extensionECH {
						b.AddBytes(zeros[:8])
					} else {
						b.AddBytes(extData)
					}
				})
			}
		})
	})

	encodedData, err := b.Bytes()
	if err == errIllegalParameter {
		return nil // Input malformed
	} else if err != nil {
		panic(err) // Host encountered internal error
	}

	return encodedData
}

// processClientHelloExtensions interprets data as a ClientHello and applies a
// function proc to each extension. Returns a bool indicating whether parsing
// succeeded.
func processClientHelloExtensions(data []byte, proc func(ext uint16, extData cryptobyte.String) bool) bool {
	_, extensionsData := splitClientHelloExtensions(data)
	if extensionsData == nil {
		return false
	}

	s := cryptobyte.String(extensionsData)
	if s.Empty() {
		// Extensions field not present.
		return true
	}

	var extensions cryptobyte.String
	if !s.ReadUint16LengthPrefixed(&extensions) || !s.Empty() {
		return false
	}

	for !extensions.Empty() {
		var ext uint16
		var extData cryptobyte.String
		if !extensions.ReadUint16(&ext) ||
			!extensions.ReadUint16LengthPrefixed(&extData) {
			return false
		}
		if ok := proc(ext, extData); !ok {
			return false
		}
	}
	return true
}

// splitClientHelloExtensions interprets data as a ClientHello message and
// returns two strings: the first contains the start of the ClientHello up to
// the start of the extensions; and the second is the length-prefixed
// extensions. Returns (nil, nil) if parsing of data fails.
func splitClientHelloExtensions(data []byte) ([]byte, []byte) {
	s := cryptobyte.String(data)

	var ignored uint16
	var t cryptobyte.String
	if !s.Skip(4) || // message type and uint24 length field
		!s.ReadUint16(&ignored) || !s.Skip(32) || // vers, random
		!s.ReadUint8LengthPrefixed(&t) { // session_id
		return nil, nil
	}

	if !s.ReadUint16LengthPrefixed(&t) { // cipher_suites
		return nil, nil
	}

	if !s.ReadUint8LengthPrefixed(&t) { // compression_methods
		return nil, nil
	}

	return data[:len(data)-len(s)], s
}

// TODO(cjpatton): Handle public name as described in draft-ietf-tls-esni-13,
// Section 4.
//
// TODO(cjpatton): Implement ECH config extensions as described in
// draft-ietf-tls-esni-13, Section 4.1.
func (c *Config) echSelectConfig() *ECHConfig {
	for _, echConfig := range c.ClientECHConfigs {
		if _, err := echConfig.selectSuite(); err == nil &&
			echConfig.version == extensionECH {
			return &echConfig
		}
	}
	return nil
}

func (c *Config) echCanOffer() bool {
	if c == nil {
		return false
	}
	return c.ECHEnabled &&
		c.echSelectConfig() != nil &&
		c.maxSupportedVersion(roleClient) >= VersionTLS13
}

func (c *Config) echCanAccept() bool {
	if c == nil {
		return false
	}
	return c.ECHEnabled &&
		c.ServerECHProvider != nil &&
		c.maxSupportedVersion(roleServer) >= VersionTLS13
}

// echOuterExtensions returns the list of extensions of the ClientHelloOuter
// that will be incorporated into the CleintHelloInner.
func echOuterExtensions() []uint16 {
	// NOTE(cjpatton): It would be nice to incorporate more extensions, but
	// "key_share" is the last extension to appear in the ClientHello before
	// "pre_shared_key". As a result, the only contiguous sequence of outer
	// extensions that contains "key_share" is "key_share" itself. Note that
	// we cannot change the order of extensions in the ClientHello, as the
	// unit tests expect "key_share" to be the second to last extension.
	outerExtensions := []uint16{extensionKeyShare}
	if testingECHOuterExtMany {
		// NOTE(cjpatton): Incorporating this particular sequence does not
		// yield significant savings. However, it's useful to test that our
		// server correctly handles a sequence of compressed extensions and
		// not just one.
		outerExtensions = []uint16{
			extensionStatusRequest,
			extensionSupportedCurves,
			extensionSupportedPoints,
		}
	} else if testingECHOuterExtNone {
		outerExtensions = []uint16{}
	}

	return outerExtensions
}

func echCopyExtensionFromClientHelloInner(hello, helloInner *clientHelloMsg, ext uint16) {
	switch ext {
	case extensionStatusRequest:
		hello.ocspStapling = helloInner.ocspStapling
	case extensionSupportedCurves:
		hello.supportedCurves = helloInner.supportedCurves
	case extensionSupportedPoints:
		hello.supportedPoints = helloInner.supportedPoints
	case extensionKeyShare:
		hello.keyShares = helloInner.keyShares
	default:
		panic(fmt.Errorf("tried to copy unrecognized extension: %04x", ext))
	}
}
