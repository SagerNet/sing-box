//go:build nullvpn
// +build nullvpn

// Package nullvpn defines the NullVPN build constraint.
// Use build tag `nullvpn` to activate the stripped NullVPN feature set.
// This tag is consumed by Makefile.nullvpn and CI/CD pipeline.
//
// Retained features under this tag:
//   - VLESS inbound/outbound
//   - TUN inbound (Android)
//   - Mixed (SOCKS5+HTTP) inbound
//   - ShadowSocks outbound (fallback)
//   - XTLS/Reality transport
//   - DNS routing
//   - Rule-based routing (GeoIP/domain)
//
// Excluded features under this tag:
//   - Hysteria / Hysteria2
//   - NaïveProxy
//   - Tor
//   - SSH
//   - Trojan
//   - ShadowTLS
//   - VMess
//   - WireGuard (handled by amneziawg-go stack)
package nullvpn
