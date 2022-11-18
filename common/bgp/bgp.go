package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	api "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/apiutil"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type BgpAPI interface {
	Lookup(ip netip.Addr, ipv4, ipv6 bool) (*BgpRoute, error)
}

type BgpRoute struct {
	IP    netip.Addr
	Attrs []BgpAttr
}

type BgpAttr struct {
	ASN       []string
	Community []string
}

func NewBgp(ctx context.Context, op option.BgpOptions, logger log.ContextLogger) (BgpAPI, error) {
	err := checkBGPOption(&op)
	if err != nil {
		logger.Debug("bgp config error:", err)
		return nil, E.New("check the bgp configuration error")
	}
	if op.Peer != nil && op.Peer.Enable {
		return newBgpServer(ctx, op, logger)
	} else {
		return newBgpGrpc(ctx, op, logger)
	}
}

func checkBGPOption(op *option.BgpOptions) error {
	if len(op.Address) <= 0 {
		return E.New("bgp service address is not set")
	} else {
		_, err := netip.ParseAddr(op.Address)
		if err != nil {
			return E.New("bgp Add legal IP address")
		}
	}
	if op.Peer != nil && op.Peer.Enable {
		if op.Port == 0 {
			op.Port = -1
		}
		if op.Peer != nil {
			if op.Peer.LocalAsn == 0 {
				return E.New("bgp local ASN is not set")
			}
			if op.Peer.PeerAsn == 0 {
				return E.New("bgp remote ASN not set")
			}
			if len(op.Peer.NeighborAddress) <= 0 {
				return E.New("bgp remote address not set")
			}
		} else {
			return E.New("bgp peer must be set when running in serviced mode")
		}
	} else {
		if op.Port == 0 {
			op.Port = 50051
		}
	}
	return nil
}

func bgpPath(ip netip.Addr, ipv4, ipv6 bool) *api.ListPathRequest {
	family := &api.Family{
		Safi: api.Family_SAFI_UNICAST,
	}
	if ipv4 {
		family.Afi = api.Family_AFI_IP
	}
	if ipv6 {
		family.Afi = api.Family_AFI_IP6
	}
	return &api.ListPathRequest{
		TableType: api.TableType_GLOBAL,
		Family:    family,
		Prefixes: []*api.TableLookupPrefix{{
			Prefix: ip.String(),
			Type:   api.TableLookupPrefix_EXACT,
		}},
		SortType: api.ListPathRequest_PREFIX,
	}
}

func pathAttr(rids ...*api.Destination) ([]BgpAttr, error) {
	bgpattrs := make([]BgpAttr, 0)
	for _, dest := range rids {
		for _, p := range dest.Paths {
			attrs, _ := apiutil.GetNativePathAttributes(p)
			asns := make([]string, 0)
			communitys := make([]string, 0)
			for _, attr := range attrs {
				if attr.GetType() == bgp.BGP_ATTR_TYPE_AS_PATH {
					asns = append(asns, strings.Split(attr.String(), " ")...)
				}
				if attr.GetType() == bgp.BGP_ATTR_TYPE_COMMUNITIES {
					communitys = append(communitys, communities(attr)...)
				}
			}
			attr := BgpAttr{
				ASN:       asns,
				Community: communitys,
			}
			bgpattrs = append(bgpattrs, attr)
		}
	}
	return bgpattrs, nil
}

func communities(value bgp.PathAttributeInterface) []string {
	p, _ := value.(*bgp.PathAttributeCommunities)
	l := make([]string, 0, len(p.Value))
	for _, v := range p.Value {
		n, ok := bgp.WellKnownCommunityNameMap[bgp.WellKnownCommunity(v)]
		if ok {
			l = append(l, n)
		} else {
			l = append(l, fmt.Sprintf("%d:%d", (0xffff0000&v)>>16, 0xffff&v))
		}
	}
	return l
}
