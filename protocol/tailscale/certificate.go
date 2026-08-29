//go:build with_gvisor

package tailscale

import (
	"context"
	"time"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

func (t *Endpoint) GetTailscaleCertificate(ctx context.Context, domain string, minValidity time.Duration) ([]byte, []byte, error) {
	if !t.started.Load() {
		return nil, nil, E.New("Tailscale is not ready yet")
	}
	certificatePEM, privateKeyPEM, err := common.Must1(t.server.LocalClient()).CertPairWithValidity(ctx, domain, minValidity)
	if err != nil {
		return nil, nil, E.Cause(err, "tailscale certificate")
	}
	return certificatePEM, privateKeyPEM, nil
}
