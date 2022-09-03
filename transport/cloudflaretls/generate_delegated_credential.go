// Copyright 2022 Cloudflare, Inc. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

//go:build ignore

// Generate a delegated credential with the given signature scheme, signed with
// the given x.509 key pair. Outputs to 'dc.cred' and 'dckey.pem' and will
// overwrite existing files.

// Example usage:
// generate_delegated_credential -cert-path cert.pem -key-path key.pem -signature-scheme Ed25519 -duration 24h

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	circlSign "github.com/cloudflare/circl/sign"
)

var (
	validFor        = flag.Duration("duration", 5*24*time.Hour, "Duration that credential is valid for")
	signatureScheme = flag.String("signature-scheme", "", "The signature scheme used by the DC")
	certPath        = flag.String("cert-path", "./cert.pem", "Path to signing cert")
	keyPath         = flag.String("key-path", "./key.pem", "Path to signing key")
	isClient        = flag.Bool("client-dc", false, "Create a client Delegated Credential")
	outPath         = flag.String("out-path", "./", "Path to output directory")
)

var SigStringMap = map[string]tls.SignatureScheme{
	// ECDSA algorithms. Only constrained to a specific curve in TLS 1.3.
	"ECDSAWithP256AndSHA256": tls.ECDSAWithP256AndSHA256,
	"ECDSAWithP384AndSHA384": tls.ECDSAWithP384AndSHA384,
	"ECDSAWithP521AndSHA512": tls.ECDSAWithP521AndSHA512,

	// EdDSA algorithms.
	"Ed25519": tls.Ed25519,
}

func main() {
	flag.Parse()
	sa := SigStringMap[*signatureScheme]

	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatalf("Failed to load certificate and key: %v", err)
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatalf("Failed to parse leaf certificate: %v", err)
	}

	validTime := time.Since(cert.Leaf.NotBefore) + *validFor
	dc, priv, err := tls.NewDelegatedCredential(&cert, sa, validTime, *isClient)
	if err != nil {
		log.Fatalf("Failed to create a DC: %v\n", err)
	}
	dcBytes, err := dc.Marshal()
	if err != nil {
		log.Fatalf("Failed to marshal DC: %v\n", err)
	}

	DCOut, err := os.Create(filepath.Join(*outPath, "dc.cred"))
	if err != nil {
		log.Fatalf("Failed to open dc.cred for writing: %v", err)
	}

	DCOut.Write(dcBytes)
	if err := DCOut.Close(); err != nil {
		log.Fatalf("Error closing dc.cred: %v", err)
	}
	log.Print("wrote dc.cred\n")

	derBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Failed to marshal DC private key: %v\n", err)
	}

	DCKeyOut, err := os.Create(filepath.Join(*outPath, "dckey.pem"))
	if err != nil {
		log.Fatalf("Failed to open dckey.pem for writing: %v", err)
	}

	if err := pem.Encode(DCKeyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to dckey.pem: %v\n", err)
	}
	if err := DCKeyOut.Close(); err != nil {
		log.Fatalf("Error closing dckey.pem: %v\n", err)
	}
	log.Print("wrote dckey.pem\n")

	fmt.Println("Success")
}

// Copied from tls.go, because it's private.
func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey, circlSign.PrivateKey:
			return key, nil
		default:
			return nil, errors.New("tls: found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	return nil, errors.New("tls: failed to parse private key")
}
