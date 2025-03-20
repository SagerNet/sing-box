package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/spf13/cobra"
	"software.sslmate.com/src/go-pkcs12"
)

var (
	flagGenerateCAName           string
	flagGenerateCAPKCS12Password string
	flagGenerateOutput           string
)

var commandGenerateCAKeyPair = &cobra.Command{
	Use:   "ca-keypair",
	Short: "Generate CA key pair",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := generateCAKeyPair()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandGenerateCAKeyPair.Flags().StringVarP(&flagGenerateCAName, "name", "n", "", "Set custom CA name")
	commandGenerateCAKeyPair.Flags().StringVarP(&flagGenerateCAPKCS12Password, "p12-password", "p", "", "Set custom PKCS12 password")
	commandGenerateCAKeyPair.Flags().StringVarP(&flagGenerateOutput, "output", "o", ".", "Set output directory")
	commandGenerate.AddCommand(commandGenerateCAKeyPair)
}

func generateCAKeyPair() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}
	spkiASN1, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	if err != nil {
		return err
	}
	skid := sha1.Sum(spki.SubjectPublicKey.Bytes)
	var caName string
	if flagGenerateCAName != "" {
		caName = flagGenerateCAName
	} else {
		caName = "sing-box Generated CA " + strings.ToUpper(hex.EncodeToString(skid[:4]))
	}
	caTpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{caName},
			CommonName:   caName,
		},
		SubjectKeyId:          skid[:],
		NotAfter:              time.Now().AddDate(10, 0, 0),
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	publicDer, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, privateKey.Public(), privateKey)
	var caPassword string
	if flagGenerateCAPKCS12Password != "" {
		caPassword = flagGenerateCAPKCS12Password
	} else {
		caPassword = strings.ToUpper(hex.EncodeToString(skid[:4]))
	}
	caTpl.Raw = publicDer
	p12Bytes, err := pkcs12.Modern.Encode(privateKey, caTpl, nil, caPassword)
	if err != nil {
		return err
	}
	privateDer, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	os.WriteFile(filepath.Join(flagGenerateOutput, caName+".pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicDer}), 0o644)
	os.WriteFile(filepath.Join(flagGenerateOutput, caName+".private.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDer}), 0o644)
	os.WriteFile(filepath.Join(flagGenerateOutput, caName+".crt"), publicDer, 0o644)
	os.WriteFile(filepath.Join(flagGenerateOutput, caName+".p12"), p12Bytes, 0o644)
	var tlsDecryptionOptions option.TLSDecryptionOptions
	tlsDecryptionOptions.Enabled = true
	tlsDecryptionOptions.KeyPair = base64.StdEncoding.EncodeToString(p12Bytes)
	tlsDecryptionOptions.KeyPairPassword = caPassword
	var certificateOptions option.CertificateOptions
	certificateOptions.TLSDecryption = &tlsDecryptionOptions
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(certificateOptions)
}
