package dns_test

import (
	"context"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	client := dns.NewClient(option.DNSClientOptions{})
	dnsTransport := dns.NewTCPTransport(context.Background(), N.SystemDialer, log.NewNopLogger(), M.ParseSocksaddr("1.0.0.1:53"))
	response, err := client.Exchange(ctx, dnsTransport, makeQuery())
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	response, err = client.Exchange(ctx, dnsTransport, makeQuery())
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	addresses, err := client.Lookup(ctx, dnsTransport, "www.google.com", C.DomainStrategyAsIS)
	require.NoError(t, err)
	require.NotEmpty(t, addresses, "no answers")
	cancel()
}
