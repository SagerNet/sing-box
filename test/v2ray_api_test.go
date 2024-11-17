package main

/*
import (
	"context"
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/v2rayapi"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

func TestV2RayAPI(t *testing.T) {
	i := startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
				Tag:  "out",
			},
		},
		Experimental: &option.ExperimentalOptions{
			V2RayAPI: &option.V2RayAPIOptions{
				Listen: "127.0.0.1:8080",
				Stats: &option.V2RayStatsServiceOptions{
					Enabled:   true,
					Inbounds:  []string{"in"},
					Outbounds: []string{"out"},
				},
			},
		},
	})
	testSuit(t, clientPort, testPort)
	statsService := i.Router().V2RayServer().StatsService()
	require.NotNil(t, statsService)
	response, err := statsService.(v2rayapi.StatsServiceServer).QueryStats(context.Background(), &v2rayapi.QueryStatsRequest{Regexp: true, Patterns: []string{".*"}})
	require.NoError(t, err)
	count := response.Stat[0].Value
	require.Equal(t, len(response.Stat), 4)
	for _, stat := range response.Stat {
		require.Equal(t, count, stat.Value)
	}
}
*/
