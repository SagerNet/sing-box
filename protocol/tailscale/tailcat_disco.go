//go:build with_tailscale

package tailscale

import (
	"fmt"
	"net/netip"

	"github.com/sagernet/tailscale/types/key"

	"go4.org/mem"
)

// Meow messages are raw DERP packets, not disco frames: 4-byte magic,
// 1-byte type, then the payload.
var tailcatMeowMagic = [4]byte{'m', 'e', 'o', 'w'}

const (
	tailcatMeowTypePing = 0x01
	tailcatMeowTypePong = 0x02
)

func isTailcatMeowPacket(packet []byte) bool {
	return len(packet) >= 4 && [4]byte(packet[:4]) == tailcatMeowMagic
}

func isTailcatMeowedPacket(packet []byte) bool {
	return len(packet) >= 5 && [4]byte(packet[:4]) == tailcatMeowMagic && packet[4] == tailcatMeowTypePong
}

func encodeTailcatMeowPing(nodeKey key.NodePublic, discoKey key.DiscoPublic) []byte {
	packet := make([]byte, 0, 4+1+key.NodePublicRawLen+key.DiscoPublicRawLen)
	packet = append(packet, tailcatMeowMagic[:]...)
	packet = append(packet, tailcatMeowTypePing)
	packet = nodeKey.AppendTo(packet)
	packet = discoKey.AppendTo(packet)
	return packet
}

func encodeTailcatMeowed(sessionID [16]byte) []byte {
	packet := make([]byte, 0, 5+len(sessionID))
	packet = append(packet, tailcatMeowMagic[:]...)
	packet = append(packet, tailcatMeowTypePong)
	return append(packet, sessionID[:]...)
}

func parseTailcatMeowPing(packet []byte) (key.NodePublic, key.DiscoPublic, bool) {
	if len(packet) < 4+1+key.NodePublicRawLen+key.DiscoPublicRawLen || packet[4] != tailcatMeowTypePing {
		return key.NodePublic{}, key.DiscoPublic{}, false
	}
	discoKey := key.DiscoPublicFromRaw32(mem.B(packet[5+key.NodePublicRawLen : 5+key.NodePublicRawLen+key.DiscoPublicRawLen]))
	if discoKey.IsZero() {
		return key.NodePublic{}, key.DiscoPublic{}, false
	}
	return key.NodePublicFromRaw32(mem.B(packet[5 : 5+key.NodePublicRawLen])), discoKey, true
}

func tailcatDiscoPrivateForNode(nodeKey key.NodePrivate) key.DiscoPrivate {
	discoKey := TailcatDiscoPrivateKey(nodeKey.Raw32())
	return key.DiscoPrivateFromRaw32(mem.B(discoKey[:]))
}

func tailcatAddressForKey(nodeKey key.NodePublic) netip.Addr {
	return TailcatAddress([32]byte(nodeKey.AppendTo(nil)))
}

// The text form is the supported way to build a node key; parsing it
// back cannot fail for a 32-byte input.
func tailcatNodePrivate(privateKey [32]byte) key.NodePrivate {
	var nodeKey key.NodePrivate
	err := nodeKey.UnmarshalText(fmt.Appendf(nil, "privkey:%x", privateKey[:]))
	if err != nil {
		panic(err)
	}
	return nodeKey
}

func tailcatPrefixForKey(nodeKey key.NodePublic) netip.Prefix {
	address := tailcatAddressForKey(nodeKey)
	return netip.PrefixFrom(address, address.BitLen())
}

var tailcatNAT64Prefix = netip.MustParsePrefix("64:ff9b::/96")

func tailcatMapNAT64(address netip.Addr) netip.Addr {
	if !address.Is4() {
		return address
	}
	mapped := tailcatNAT64Prefix.Addr().As16()
	address4 := address.As4()
	copy(mapped[12:], address4[:])
	return netip.AddrFrom16(mapped)
}

func tailcatUnmapNAT64(address netip.Addr) netip.Addr {
	if !tailcatNAT64Prefix.Contains(address) {
		return address
	}
	mapped := address.As16()
	return netip.AddrFrom4([4]byte(mapped[12:]))
}
