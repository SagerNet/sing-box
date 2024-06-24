package geosite_test

import (
	"bytes"
	"testing"

	"github.com/sagernet/sing-box/common/geosite"

	"github.com/stretchr/testify/require"
)

func TestGeosite(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := geosite.Write(&buffer, map[string][]geosite.Item{
		"test": {
			{
				Type:  geosite.RuleTypeDomain,
				Value: "example.org",
			},
		},
	})
	require.NoError(t, err)
	reader, codes, err := geosite.NewReader(bytes.NewReader(buffer.Bytes()))
	require.NoError(t, err)
	require.Equal(t, []string{"test"}, codes)
	items, err := reader.Read("test")
	require.NoError(t, err)
	require.Equal(t, []geosite.Item{{
		Type:  geosite.RuleTypeDomain,
		Value: "example.org",
	}}, items)
}
