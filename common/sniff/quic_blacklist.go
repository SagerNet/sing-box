package sniff

import (
	"crypto/tls"

	"github.com/sagernet/sing-box/common/ja3"
)

// Chromium sends separate client hello packets, but UQUIC has not yet implemented this behavior
// The cronet without this behavior does not have version 115
var uQUICChrome115 = &ja3.ClientHello{
	Version:             tls.VersionTLS12,
	CipherSuites:        []uint16{4865, 4866, 4867},
	Extensions:          []uint16{0, 10, 13, 16, 27, 43, 45, 51, 57, 17513},
	EllipticCurves:      []uint16{29, 23, 24},
	SignatureAlgorithms: []uint16{1027, 2052, 1025, 1283, 2053, 1281, 2054, 1537, 513},
}

func maybeUQUIC(fingerprint *ja3.ClientHello) bool {
	return !uQUICChrome115.Equals(fingerprint, true)
}
