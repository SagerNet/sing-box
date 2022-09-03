// Copyright 2020 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package tls

import (
	"errors"
	"fmt"
	"io"

	"github.com/cloudflare/circl/hpke"
	"github.com/cloudflare/circl/kem"
	"golang.org/x/crypto/cryptobyte"
)

// ECHConfig represents an ECH configuration.
type ECHConfig struct {
	pk  kem.PublicKey
	raw []byte

	// Parsed from raw
	version           uint16
	configId          uint8
	rawPublicName     []byte
	rawPublicKey      []byte
	kemId             uint16
	suites            []hpkeSymmetricCipherSuite
	maxNameLen        uint8
	ignoredExtensions []byte
}

// UnmarshalECHConfigs parses a sequence of ECH configurations.
func UnmarshalECHConfigs(raw []byte) ([]ECHConfig, error) {
	var (
		err         error
		config      ECHConfig
		t, contents cryptobyte.String
	)
	configs := make([]ECHConfig, 0)
	s := cryptobyte.String(raw)
	if !s.ReadUint16LengthPrefixed(&t) || !s.Empty() {
		return configs, errors.New("error parsing configs")
	}
	raw = raw[2:]
ConfigsLoop:
	for !t.Empty() {
		l := len(t)
		if !t.ReadUint16(&config.version) ||
			!t.ReadUint16LengthPrefixed(&contents) {
			return nil, errors.New("error parsing config")
		}
		n := l - len(t)
		config.raw = raw[:n]
		raw = raw[n:]

		if config.version != extensionECH {
			continue ConfigsLoop
		}
		if !readConfigContents(&contents, &config) {
			return nil, errors.New("error parsing config contents")
		}

		kem := hpke.KEM(config.kemId)
		if !kem.IsValid() {
			continue ConfigsLoop
		}
		config.pk, err = kem.Scheme().UnmarshalBinaryPublicKey(config.rawPublicKey)
		if err != nil {
			return nil, fmt.Errorf("error parsing public key: %s", err)
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func echMarshalConfigs(configs []ECHConfig) ([]byte, error) {
	var b cryptobyte.Builder
	b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
		for _, config := range configs {
			if config.raw == nil {
				panic("config.raw not set")
			}
			b.AddBytes(config.raw)
		}
	})
	return b.Bytes()
}

func readConfigContents(contents *cryptobyte.String, config *ECHConfig) bool {
	var t cryptobyte.String
	if !contents.ReadUint8(&config.configId) ||
		!contents.ReadUint16(&config.kemId) ||
		!contents.ReadUint16LengthPrefixed(&t) ||
		!t.ReadBytes(&config.rawPublicKey, len(t)) ||
		!contents.ReadUint16LengthPrefixed(&t) ||
		len(t)%4 != 0 {
		return false
	}

	config.suites = nil
	for !t.Empty() {
		var kdfId, aeadId uint16
		if !t.ReadUint16(&kdfId) || !t.ReadUint16(&aeadId) {
			// This indicates an internal bug.
			panic("internal error while parsing contents.cipher_suites")
		}
		config.suites = append(config.suites, hpkeSymmetricCipherSuite{kdfId, aeadId})
	}

	if !contents.ReadUint8(&config.maxNameLen) ||
		!contents.ReadUint8LengthPrefixed(&t) ||
		!t.ReadBytes(&config.rawPublicName, len(t)) ||
		!contents.ReadUint16LengthPrefixed(&t) ||
		!t.ReadBytes(&config.ignoredExtensions, len(t)) ||
		!contents.Empty() {
		return false
	}
	return true
}

// setupSealer generates the client's HPKE context for use with the ECH
// extension. It returns the context and corresponding encapsulated key.
func (config *ECHConfig) setupSealer(rand io.Reader) (enc []byte, sealer hpke.Sealer, err error) {
	if config.raw == nil {
		panic("config.raw not set")
	}
	hpkeSuite, err := config.selectSuite()
	if err != nil {
		return nil, nil, err
	}
	info := append(append([]byte(echHpkeInfoSetup), 0), config.raw...)
	sender, err := hpkeSuite.NewSender(config.pk, info)
	if err != nil {
		return nil, nil, err
	}
	return sender.Setup(rand)
}

// isPeerCipherSuiteSupported returns true if this configuration indicates
// support for the given ciphersuite.
func (config *ECHConfig) isPeerCipherSuiteSupported(suite hpkeSymmetricCipherSuite) bool {
	for _, configSuite := range config.suites {
		if suite == configSuite {
			return true
		}
	}
	return false
}

// selectSuite returns the first ciphersuite indicated by this
// configuration that is supported by the caller.
func (config *ECHConfig) selectSuite() (hpke.Suite, error) {
	for _, suite := range config.suites {
		hpkeSuite, err := hpkeAssembleSuite(
			config.kemId,
			suite.kdfId,
			suite.aeadId,
		)
		if err == nil {
			return hpkeSuite, nil
		}
	}
	return hpke.Suite{}, errors.New("could not negotiate a ciphersuite")
}
