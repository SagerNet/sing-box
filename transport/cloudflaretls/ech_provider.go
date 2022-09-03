// Copyright 2020 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package tls

import (
	"errors"
	"fmt"

	"github.com/cloudflare/circl/hpke"
	"github.com/cloudflare/circl/kem"
	"golang.org/x/crypto/cryptobyte"
)

// ECHProvider specifies the interface of an ECH service provider that decrypts
// the ECH payload on behalf of the client-facing server. It also defines the
// set of acceptable ECH configurations.
type ECHProvider interface {
	// GetDecryptionContext attempts to construct the HPKE context used by the
	// client-facing server for decryption. (See draft-irtf-cfrg-hpke-07,
	// Section 5.2.)
	//
	// handle encodes the parameters of the client's "encrypted_client_hello"
	// extension that are needed to construct the context. Since
	// draft-ietf-tls-esni-10 these are the ECH cipher suite, the identity of
	// the ECH configuration, and the encapsulated key.
	//
	// version is the version of ECH indicated by the client.
	//
	// res.Status == ECHProviderStatusSuccess indicates the call was successful
	// and the caller may proceed. res.Context is set.
	//
	// res.Status == ECHProviderStatusReject indicates the caller must reject
	// ECH. res.RetryConfigs may be set.
	//
	// res.Status == ECHProviderStatusAbort indicates the caller should abort
	// the handshake.  Note that, in some cases, it's appropriate to reject
	// rather than abort. In particular, aborting with "illegal_parameter" might
	// "stick out". res.Alert and res.Error are set.
	GetDecryptionContext(handle []byte, version uint16) (res ECHProviderResult)
}

// ECHProviderStatus is the status of the ECH provider's response.
type ECHProviderStatus uint

const (
	ECHProviderSuccess ECHProviderStatus = 0
	ECHProviderReject                    = 1
	ECHProviderAbort                     = 2

	errHPKEInvalidPublicKey = "hpke: invalid KEM public key"
)

// ECHProviderResult represents the result of invoking the ECH provider.
type ECHProviderResult struct {
	Status ECHProviderStatus

	// Alert is the TLS alert sent by the caller when aborting the handshake.
	Alert uint8

	// Error is the error propagated by the caller when aborting the handshake.
	Error error

	// RetryConfigs is the sequence of ECH configs to offer to the client for
	// retrying the handshake. This may be set in case of success or rejection.
	RetryConfigs []byte

	// Context is the server's HPKE context. This is set if ECH is not rejected
	// by the provider and no error was reported. The data has the following
	// format (in TLS syntax):
	//
	// enum { sealer(0), opener(1) } HpkeRole;
	//
	// struct {
	//     HpkeRole role;
	//     HpkeKemId kem_id;   // as defined in draft-irtf-cfrg-hpke-07
	//     HpkeKdfId kdf_id;   // as defined in draft-irtf-cfrg-hpke-07
	//     HpkeAeadId aead_id; // as defined in draft-irtf-cfrg-hpke-07
	//     opaque exporter_secret<0..255>;
	//     opaque key<0..255>;
	//     opaque base_nonce<0..255>;
	//     opaque seq<0..255>;
	// } HpkeContext;
	Context []byte
}

// EXP_ECHKeySet implements the ECHProvider interface for a sequence of ECH keys.
//
// NOTE: This API is EXPERIMENTAL and subject to change.
type EXP_ECHKeySet struct {
	// The serialized ECHConfigs, in order of the server's preference.
	configs []byte

	// Maps a configuration identifier to its secret key.
	sk map[uint8]EXP_ECHKey
}

// EXP_NewECHKeySet constructs an EXP_ECHKeySet.
func EXP_NewECHKeySet(keys []EXP_ECHKey) (*EXP_ECHKeySet, error) {
	if len(keys) > 255 {
		return nil, fmt.Errorf("tls: ech provider: unable to support more than 255 ECH configurations at once")
	}

	keySet := new(EXP_ECHKeySet)
	keySet.sk = make(map[uint8]EXP_ECHKey)
	configs := make([]byte, 0)
	for _, key := range keys {
		if _, ok := keySet.sk[key.config.configId]; ok {
			return nil, fmt.Errorf("tls: ech provider: ECH config conflict for configId %d", key.config.configId)
		}

		keySet.sk[key.config.configId] = key
		configs = append(configs, key.config.raw...)
	}

	var b cryptobyte.Builder
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		b.AddBytes(configs)
	})
	keySet.configs = b.BytesOrPanic()

	return keySet, nil
}

// GetDecryptionContext is required by the ECHProvider interface.
func (keySet *EXP_ECHKeySet) GetDecryptionContext(rawHandle []byte, version uint16) (res ECHProviderResult) {
	// Propagate retry configurations regardless of the result. The caller sends
	// these to the clients only if it rejects.
	res.RetryConfigs = keySet.configs

	// Ensure we know how to proceed, i.e., the caller has indicated a supported
	// version of ECH. Currently only draft-ietf-tls-esni-13 is supported.
	if version != extensionECH {
		res.Status = ECHProviderAbort
		res.Alert = uint8(alertInternalError)
		res.Error = errors.New("version not supported")
		return // Abort
	}

	// Parse the handle.
	s := cryptobyte.String(rawHandle)
	handle := new(echContextHandle)
	if !echReadContextHandle(&s, handle) || !s.Empty() {
		// This is the result of a client-side error. However, aborting with
		// "illegal_parameter" would stick out, so we reject instead.
		res.Status = ECHProviderReject
		res.RetryConfigs = keySet.configs
		return // Reject
	}
	handle.raw = rawHandle

	// Look up the secret key for the configuration indicated by the client.
	key, ok := keySet.sk[handle.configId]
	if !ok {
		res.Status = ECHProviderReject
		res.RetryConfigs = keySet.configs
		return // Reject
	}

	// Ensure that support for the selected ciphersuite is indicated by the
	// configuration.
	suite := handle.suite
	if !key.config.isPeerCipherSuiteSupported(suite) {
		// This is the result of a client-side error. However, aborting with
		// "illegal_parameter" would stick out, so we reject instead.
		res.Status = ECHProviderReject
		res.RetryConfigs = keySet.configs
		return // Reject
	}

	// Ensure the version indicated by the client matches the version supported
	// by the configuration.
	if version != key.config.version {
		// This is the result of a client-side error. However, aborting with
		// "illegal_parameter" would stick out, so we reject instead.
		res.Status = ECHProviderReject
		res.RetryConfigs = keySet.configs
		return // Reject
	}

	// Compute the decryption context.
	opener, err := key.setupOpener(handle.enc, suite)
	if err != nil {
		if err.Error() == errHPKEInvalidPublicKey {
			// This occurs if the KEM algorithm used to generate handle.enc is
			// not the same as the KEM algorithm of the key. One way this can
			// happen is if the client sent a GREASE ECH extension with a
			// config_id that happens to match a known config, but which uses a
			// different KEM algorithm.
			res.Status = ECHProviderReject
			res.RetryConfigs = keySet.configs
			return // Reject
		}

		res.Status = ECHProviderAbort
		res.Alert = uint8(alertInternalError)
		res.Error = err
		return // Abort
	}

	// Serialize the decryption context.
	res.Context, err = opener.MarshalBinary()
	if err != nil {
		res.Status = ECHProviderAbort
		res.Alert = uint8(alertInternalError)
		res.Error = err
		return // Abort
	}

	res.Status = ECHProviderSuccess
	return // Success
}

// EXP_ECHKey represents an ECH key and its corresponding configuration. The
// encoding of an ECH Key has the format defined below (in TLS syntax). Note
// that the ECH standard does not specify this format.
//
//	struct {
//	    opaque sk<0..2^16-1>;
//	    ECHConfig config<0..2^16>; // draft-ietf-tls-esni-13
//	} ECHKey;
type EXP_ECHKey struct {
	sk     kem.PrivateKey
	config ECHConfig
}

// EXP_UnmarshalECHKeys parses a sequence of ECH keys.
func EXP_UnmarshalECHKeys(raw []byte) ([]EXP_ECHKey, error) {
	var (
		err                  error
		key                  EXP_ECHKey
		sk, config, contents cryptobyte.String
	)
	s := cryptobyte.String(raw)
	keys := make([]EXP_ECHKey, 0)
KeysLoop:
	for !s.Empty() {
		if !s.ReadUint16LengthPrefixed(&sk) ||
			!s.ReadUint16LengthPrefixed(&config) {
			return nil, errors.New("error parsing key")
		}

		key.config.raw = config
		if !config.ReadUint16(&key.config.version) ||
			!config.ReadUint16LengthPrefixed(&contents) ||
			!config.Empty() {
			return nil, errors.New("error parsing config")
		}

		if key.config.version != extensionECH {
			continue KeysLoop
		}
		if !readConfigContents(&contents, &key.config) {
			return nil, errors.New("error parsing config contents")
		}

		for _, suite := range key.config.suites {
			if !hpke.KDF(suite.kdfId).IsValid() ||
				!hpke.AEAD(suite.aeadId).IsValid() {
				continue KeysLoop
			}
		}

		kem := hpke.KEM(key.config.kemId)
		if !kem.IsValid() {
			continue KeysLoop
		}
		key.config.pk, err = kem.Scheme().UnmarshalBinaryPublicKey(key.config.rawPublicKey)
		if err != nil {
			return nil, fmt.Errorf("error parsing public key: %s", err)
		}
		key.sk, err = kem.Scheme().UnmarshalBinaryPrivateKey(sk)
		if err != nil {
			return nil, fmt.Errorf("error parsing secret key: %s", err)
		}

		keys = append(keys, key)
	}
	return keys, nil
}

// setupOpener computes the HPKE context used by the server in the ECH
// extension.i
func (key *EXP_ECHKey) setupOpener(enc []byte, suite hpkeSymmetricCipherSuite) (hpke.Opener, error) {
	if key.config.raw == nil {
		panic("raw config not set")
	}
	hpkeSuite, err := hpkeAssembleSuite(
		key.config.kemId,
		suite.kdfId,
		suite.aeadId,
	)
	if err != nil {
		return nil, err
	}
	info := append(append([]byte(echHpkeInfoSetup), 0), key.config.raw...)
	receiver, err := hpkeSuite.NewReceiver(key.sk, info)
	if err != nil {
		return nil, err
	}
	return receiver.Setup(enc)
}
