package clashapi

import (
	"archive/zip"
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/gofrs/uuid/v5"
	"howett.net/plist"
)

func mitmRouter(ctx context.Context) http.Handler {
	r := chi.NewRouter()
	r.Get("/mobileconfig", getMobileConfig(ctx))
	r.Get("/crt", getCertificate(ctx))
	r.Get("/pem", getCertificatePEM(ctx))
	r.Get("/magisk", getMagiskModule(ctx))
	return r
}

func getMobileConfig(ctx context.Context) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store := service.FromContext[adapter.CertificateStore](ctx)
		if !store.TLSDecryptionEnabled() {
			http.NotFound(writer, request)
			render.PlainText(writer, request, "TLS decryption not enabled")
			return
		}
		certificate := store.TLSDecryptionCertificate()
		writer.Header().Set("Content-Type", "application/x-apple-aspen-config")
		uuidGen := common.Must1(uuid.NewV4()).String()
		mobileConfig := map[string]interface{}{
			"PayloadContent": []interface{}{
				map[string]interface{}{
					"PayloadCertificateFileName": "Certificates.cer",
					"PayloadContent":             certificate.Raw,
					"PayloadDescription":         "Adds a root certificate",
					"PayloadDisplayName":         certificate.Subject.CommonName,
					"PayloadIdentifier":          "com.apple.security.root." + uuidGen,
					"PayloadType":                "com.apple.security.root",
					"PayloadUUID":                uuidGen,
					"PayloadVersion":             1,
				},
			},
			"PayloadDisplayName":       certificate.Subject.CommonName,
			"PayloadIdentifier":        "io.nekohasekai.sfa.ca.profile." + uuidGen,
			"PayloadRemovalDisallowed": false,
			"PayloadType":              "Configuration",
			"PayloadUUID":              uuidGen,
			"PayloadVersion":           1,
		}
		encoder := plist.NewEncoder(writer)
		encoder.Indent("\t")
		encoder.Encode(mobileConfig)
	}
}

func getCertificate(ctx context.Context) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store := service.FromContext[adapter.CertificateStore](ctx)
		if !store.TLSDecryptionEnabled() {
			http.NotFound(writer, request)
			render.PlainText(writer, request, "TLS decryption not enabled")
			return
		}
		writer.Header().Set("Content-Type", "application/x-x509-ca-cert")
		writer.Header().Set("Content-Disposition", "attachment; filename=Certificate.crt")
		writer.Write(store.TLSDecryptionCertificate().Raw)
	}
}

func getCertificatePEM(ctx context.Context) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store := service.FromContext[adapter.CertificateStore](ctx)
		if !store.TLSDecryptionEnabled() {
			http.NotFound(writer, request)
			render.PlainText(writer, request, "TLS decryption not enabled")
			return
		}
		writer.Header().Set("Content-Type", "application/x-pem-file")
		writer.Header().Set("Content-Disposition", "attachment; filename=Certificate.pem")
		writer.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: store.TLSDecryptionCertificate().Raw}))
	}
}

func getMagiskModule(ctx context.Context) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		store := service.FromContext[adapter.CertificateStore](ctx)
		if !store.TLSDecryptionEnabled() {
			http.NotFound(writer, request)
			render.PlainText(writer, request, "TLS decryption not enabled")
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		writer.Header().Set("Content-Disposition", "attachment; filename="+store.TLSDecryptionCertificate().Subject.CommonName+".zip")
		createMagiskModule(writer, store.TLSDecryptionCertificate())
	}
}

func createMagiskModule(writer io.Writer, certificate *x509.Certificate) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()
	moduleProp, err := zipWriter.Create("module.prop")
	if err != nil {
		return err
	}
	_, err = moduleProp.Write([]byte(`
id=sing-box-certificate
name=` + certificate.Subject.CommonName + `
version=v0.0.1
versionCode=1
author=sing-box
description=This module adds ` + certificate.Subject.CommonName + ` to the system trust store.
`))
	if err != nil {
		return err
	}
	certificateFile, err := zipWriter.Create("system/etc/security/cacerts/" + certificate.Subject.CommonName + ".pem")
	if err != nil {
		return err
	}
	err = pem.Encode(certificateFile, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err != nil {
		return err
	}
	updateBinary, err := zipWriter.Create("META-INF/com/google/android/update-binary")
	if err != nil {
		return err
	}
	_, err = updateBinary.Write([]byte(`
#!/sbin/sh

#################
# Initialization
#################

umask 022

# echo before loading util_functions
ui_print() { echo "$1"; }

require_new_magisk() {
  ui_print "*******************************"
  ui_print " Please install Magisk v20.4+! "
  ui_print "*******************************"
  exit 1
}

#########################
# Load util_functions.sh
#########################

OUTFD=$2
ZIPFILE=$3

mount /data 2>/dev/null

[ -f /data/adb/magisk/util_functions.sh ] || require_new_magisk
. /data/adb/magisk/util_functions.sh
[ $MAGISK_VER_CODE -lt 20400 ] && require_new_magisk

install_module
exit 0
`))
	if err != nil {
		return err
	}
	updaterScript, err := zipWriter.Create("META-INF/com/google/android/updater-script")
	if err != nil {
		return err
	}
	_, err = updaterScript.Write([]byte("#MAGISK"))
	if err != nil {
		return err
	}
	return nil
}
