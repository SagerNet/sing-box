package clashapi

import (
	"context"
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
	r.Get("/certificate", getCertificate(ctx))
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
