package proxy

import "encoding/base64"

func Base64Decode(rawData string) ([]byte, error) {
	r, err := base64.RawURLEncoding.DecodeString(rawData)
	if err == nil {
		return r, nil
	}
	r, err = base64.URLEncoding.DecodeString(rawData)
	if err == nil {
		return r, nil
	}
	r, err = base64.StdEncoding.DecodeString(rawData)
	if err == nil {
		return r, nil
	}
	return nil, err
}

var allowShadowsocksMethod = map[string]bool{
	"none":                          true,
	"2022-blake3-aes-128-gcm":       true,
	"2022-blake3-aes-256-gcm":       true,
	"2022-blake3-chacha20-poly1305": true,
	"aes-128-gcm":                   true,
	"aes-192-gcm":                   true,
	"aes-256-gcm":                   true,
	"chacha20-ietf-poly1305":        true,
	"xchacha20-ietf-poly1305":       true,
	"aes-128-ctr":                   true,
	"aes-192-ctr":                   true,
	"aes-256-ctr":                   true,
	"aes-128-cfb":                   true,
	"aes-192-cfb":                   true,
	"aes-256-cfb":                   true,
	"rc4-md5":                       true,
	"chacha20-ietf":                 true,
	"xchacha20":                     true,
}

func checkShadowsocksAllowMethod(method string) bool {
	return allowShadowsocksMethod[method]
}
