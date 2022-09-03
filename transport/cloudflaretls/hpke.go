// Copyright 2020 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package tls

import (
	"errors"
	"fmt"

	"github.com/cloudflare/circl/hpke"
)

// The mandatory-to-implement HPKE cipher suite for use with the ECH extension.
var defaultHPKESuite hpke.Suite

func init() {
	var err error
	defaultHPKESuite, err = hpkeAssembleSuite(
		uint16(hpke.KEM_X25519_HKDF_SHA256),
		uint16(hpke.KDF_HKDF_SHA256),
		uint16(hpke.AEAD_AES128GCM),
	)
	if err != nil {
		panic(fmt.Sprintf("hpke: mandatory-to-implement cipher suite not supported: %s", err))
	}
}

func hpkeAssembleSuite(kemId, kdfId, aeadId uint16) (hpke.Suite, error) {
	kem := hpke.KEM(kemId)
	if !kem.IsValid() {
		return hpke.Suite{}, errors.New("KEM is not supported")
	}
	kdf := hpke.KDF(kdfId)
	if !kdf.IsValid() {
		return hpke.Suite{}, errors.New("KDF is not supported")
	}
	aead := hpke.AEAD(aeadId)
	if !aead.IsValid() {
		return hpke.Suite{}, errors.New("AEAD is not supported")
	}
	return hpke.NewSuite(kem, kdf, aead), nil
}
