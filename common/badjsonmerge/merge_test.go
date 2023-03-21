package badjsonmerge

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func TestMergeJSON(t *testing.T) {
	t.Parallel()
	options := option.Options{
		Log: &option.LogOptions{
			Level: "info",
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						Network:  []string{N.NetworkTCP},
						Outbound: "direct",
					},
				},
			},
		},
	}
	anotherOptions := option.Options{
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
				Tag:  "direct",
			},
		},
	}
	thirdOptions := option.Options{
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						Network:  []string{N.NetworkUDP},
						Outbound: "direct",
					},
				},
			},
		},
	}
	mergeOptions, err := MergeOptions(options, anotherOptions)
	require.NoError(t, err)
	mergeOptions, err = MergeOptions(thirdOptions, mergeOptions)
	require.NoError(t, err)
	require.Equal(t, "info", mergeOptions.Log.Level)
	require.Equal(t, 2, len(mergeOptions.Route.Rules))
	require.Equal(t, C.TypeDirect, mergeOptions.Outbounds[0].Type)
}
