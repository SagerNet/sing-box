//go:build with_bgp

package bgp

import (
	"context"
	"net/netip"

	api "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/server"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type bgpServer struct {
	connect *server.BgpServer
}

func newBgpServer(ctx context.Context, op option.BgpOptions, logger log.ContextLogger) (BgpAPI, error) {
	s := server.NewBgpServer(server.LoggerOption(&bgplog{logger: logger}))
	go s.Serve()

	if err := s.StartBgp(context.TODO(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        op.Peer.LocalAsn,
			RouterId:   op.Address,
			ListenPort: op.Port,
			DefaultRouteDistance: &api.DefaultRouteDistance{
				ExternalRouteDistance: uint32(api.RouteAction_REJECT),
				InternalRouteDistance: uint32(api.RouteAction_REJECT),
			},
		},
	}); err != nil {
		return nil, err
	}
	// neighbor configuration
	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: op.Peer.NeighborAddress,
			PeerAsn:         op.Peer.PeerAsn,
			AuthPassword:    op.Peer.AuthPassword,
		},
		ApplyPolicy: &api.ApplyPolicy{
			ImportPolicy: &api.PolicyAssignment{
				DefaultAction: api.RouteAction_ACCEPT,
			},
			ExportPolicy: &api.PolicyAssignment{
				DefaultAction: api.RouteAction_REJECT,
			},
		},
	}

	if err := s.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: n,
	}); err != nil {
		return nil, err
	}
	return &bgpServer{
		connect: s,
	}, nil
}

func (r *bgpServer) Lookup(ip netip.Addr, ipv4, ipv6 bool) (*BgpRoute, error) {
	attrs := make([]BgpAttr, 0)
	err := r.connect.ListPath(context.TODO(), bgpPath(ip, ipv4, ipv6), func(d *api.Destination) {
		bagattrs, err := pathAttr(d)
		if err != nil {
			return
		}
		attrs = append(attrs, bagattrs...)
	})
	if err != nil {
		return nil, err
	}
	if len(attrs) <= 0 {
		return nil, E.New("No data obtained")
	}
	return &BgpRoute{IP: ip, Attrs: attrs}, nil
}
