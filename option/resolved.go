package option

import (
	"context"
	"net/netip"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

type _ResolvedServiceOptions struct {
	ListenOptions
}

type ResolvedServiceOptions _ResolvedServiceOptions

func (r ResolvedServiceOptions) MarshalJSONContext(ctx context.Context) ([]byte, error) {
	if r.Listen != nil && netip.Addr(*r.Listen) == (netip.AddrFrom4([4]byte{127, 0, 0, 53})) {
		r.Listen = nil
	}
	if r.ListenPort == 53 {
		r.ListenPort = 0
	}
	return json.MarshalContext(ctx, (*_ResolvedServiceOptions)(&r))
}

func (r *ResolvedServiceOptions) UnmarshalJSONContext(ctx context.Context, bytes []byte) error {
	err := json.UnmarshalContextDisallowUnknownFields(ctx, bytes, (*_ResolvedServiceOptions)(r))
	if err != nil {
		return err
	}
	if r.Listen == nil {
		r.Listen = (*badoption.Addr)(common.Ptr(netip.AddrFrom4([4]byte{127, 0, 0, 53})))
	}
	if r.ListenPort == 0 {
		r.ListenPort = 53
	}
	return nil
}

type ResolvedDNSServerOptions struct {
	Service                string `json:"service"`
	AcceptDefaultResolvers bool   `json:"accept_default_resolvers,omitempty"`
	// NDots                  int                `json:"ndots,omitempty"`
	// Timeout                badoption.Duration `json:"timeout,omitempty"`
	// Attempts               int                `json:"attempts,omitempty"`
	// Rotate                 bool               `json:"rotate,omitempty"`
}
