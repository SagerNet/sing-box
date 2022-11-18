package bgp

import (
	"context"
	"fmt"
	"io"
	"net/netip"

	api "github.com/osrg/gobgp/v3/api"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type bgpGrpc struct {
	connect api.GobgpApiClient
}

func newBgpGrpc(ctx context.Context, op option.BgpOptions, logger log.ContextLogger) (BgpAPI, error) {
	target := fmt.Sprintf("%v:%v", op.Address, op.Port)
	_, err := netip.ParseAddrPort(target)
	if err != nil {
		return nil, E.New("bgp grpc connection IP address is incorrect error: ", err)
	}
	conn, err := grpc.DialContext(context.TODO(), target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, E.New("fail to connect to gobgp with error:", err)
	}
	client := api.NewGobgpApiClient(conn)
	if _, err := client.GetBgp(context.TODO(), &api.GetBgpRequest{}); err != nil {
		return nil, E.New("fail to get gobgp info with error: ", err)
	}
	return &bgpGrpc{
		connect: client,
	}, nil
}

func (r *bgpGrpc) Lookup(ip netip.Addr, ipv4, ipv6 bool) (*BgpRoute, error) {
	rib := make([]*api.Destination, 0)
	stream, err := r.connect.ListPath(context.TODO(), bgpPath(ip, ipv4, ipv6))
	if err != nil {
		return nil, err
	}
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		rib = append(rib, r.Destination)
	}
	if len(rib) <= 0 {
		return nil, err
	}
	bgpattrs, err := pathAttr(rib...)
	if err != nil {
		return nil, err
	}
	return &BgpRoute{IP: ip, Attrs: bgpattrs}, nil
}
