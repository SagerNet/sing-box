package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

type PeerConfig struct {
	destination    M.Socksaddr
	domainStrategy dns.DomainStrategy
	Endpoint       netip.AddrPort
	PublicKey      string
	PreSharedKey   string
	AllowedIPs     []string
	Reserved       [3]uint8
}

func (c PeerConfig) GenerateIpcLines() string {
	ipcLines := "\npublic_key=" + c.PublicKey
	ipcLines += "\nendpoint=" + c.Endpoint.String()
	if c.PreSharedKey != "" {
		ipcLines += "\npreshared_key=" + c.PreSharedKey
	}
	for _, allowedIP := range c.AllowedIPs {
		ipcLines += "\nallowed_ip=" + allowedIP
	}
	return ipcLines
}

func ParsePeers(options option.WireGuardOutboundOptions) ([]PeerConfig, error) {
	var peers []PeerConfig
	if len(options.Peers) > 0 {
		for peerIndex, rawPeer := range options.Peers {
			peer := PeerConfig{
				AllowedIPs: rawPeer.AllowedIPs,
			}
			destination := rawPeer.ServerOptions.Build()
			if destination.IsFqdn() {
				peer.destination = destination
				peer.domainStrategy = dns.DomainStrategy(options.DomainStrategy)
			} else {
				peer.Endpoint = destination.AddrPort()
			}
			{
				bytes, err := base64.StdEncoding.DecodeString(rawPeer.PublicKey)
				if err != nil {
					return nil, E.Cause(err, "decode public key for peer ", peerIndex)
				}
				peer.PublicKey = hex.EncodeToString(bytes)
			}
			if rawPeer.PreSharedKey != "" {
				bytes, err := base64.StdEncoding.DecodeString(rawPeer.PreSharedKey)
				if err != nil {
					return nil, E.Cause(err, "decode pre shared key for peer ", peerIndex)
				}
				peer.PreSharedKey = hex.EncodeToString(bytes)
			}
			if len(rawPeer.AllowedIPs) == 0 {
				return nil, E.New("missing allowed_ips for peer ", peerIndex)
			}
			if len(rawPeer.Reserved) > 0 {
				if len(rawPeer.Reserved) != 3 {
					return nil, E.New("invalid reserved value for peer ", peerIndex, ", required 3 bytes, got ", len(peer.Reserved))
				}
				copy(peer.Reserved[:], options.Reserved)
			}
			peers = append(peers, peer)
		}
	} else {
		peer := PeerConfig{}
		var (
			addressHas4 bool
			addressHas6 bool
		)
		for _, localAddress := range options.LocalAddress {
			if localAddress.Addr().Is4() {
				addressHas4 = true
			} else {
				addressHas6 = true
			}
		}
		if addressHas4 {
			peer.AllowedIPs = append(peer.AllowedIPs, netip.PrefixFrom(netip.IPv4Unspecified(), 0).String())
		}
		if addressHas6 {
			peer.AllowedIPs = append(peer.AllowedIPs, netip.PrefixFrom(netip.IPv6Unspecified(), 0).String())
		}
		destination := options.ServerOptions.Build()
		if destination.IsFqdn() {
			peer.destination = destination
			peer.domainStrategy = dns.DomainStrategy(options.DomainStrategy)
		} else {
			peer.Endpoint = destination.AddrPort()
		}
		{
			bytes, err := base64.StdEncoding.DecodeString(options.PeerPublicKey)
			if err != nil {
				return nil, E.Cause(err, "decode peer public key")
			}
			peer.PublicKey = hex.EncodeToString(bytes)
		}
		if options.PreSharedKey != "" {
			bytes, err := base64.StdEncoding.DecodeString(options.PreSharedKey)
			if err != nil {
				return nil, E.Cause(err, "decode pre shared key")
			}
			peer.PreSharedKey = hex.EncodeToString(bytes)
		}
		if len(options.Reserved) > 0 {
			if len(options.Reserved) != 3 {
				return nil, E.New("invalid reserved value, required 3 bytes, got ", len(peer.Reserved))
			}
			copy(peer.Reserved[:], options.Reserved)
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func ResolvePeers(ctx context.Context, router adapter.Router, peers []PeerConfig) error {
	for peerIndex, peer := range peers {
		if peer.Endpoint.IsValid() {
			continue
		}
		destinationAddresses, err := router.Lookup(ctx, peer.destination.Fqdn, peer.domainStrategy)
		if err != nil {
			if len(peers) == 1 {
				return E.Cause(err, "resolve endpoint domain")
			} else {
				return E.Cause(err, "resolve endpoint domain for peer ", peerIndex)
			}
		}
		if len(destinationAddresses) == 0 {
			return E.New("no addresses found for endpoint domain: ", peer.destination.Fqdn)
		}
		peers[peerIndex].Endpoint = netip.AddrPortFrom(destinationAddresses[0], peer.destination.Port)

	}
	return nil
}
