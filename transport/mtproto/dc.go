package mtproto

import (
	"net/netip"

	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
)

var ProductionDataCenterAddress = map[int][]netip.Addr{
	1: {
		M.ParseAddr("149.154.175.50"),
		M.ParseAddr("2001:b28:f23d:f001::a"),
	},
	2: {
		M.ParseAddr("149.154.167.51"),
		M.ParseAddr("2001:67c:04e8:f002::a"),
	},
	3: {
		M.ParseAddr("149.154.175.100"),
		M.ParseAddr("2001:b28:f23d:f003::a"),
	},
	4: {
		M.ParseAddr("149.154.167.91"),
		M.ParseAddr("2001:67c:04e8:f004::a"),
	},
	5: {
		M.ParseAddr("149.154.171.5"),
		M.ParseAddr("2001:b28:f23f:f005::a"),
	},
}

var TestDataCenterAddress = map[int][]netip.Addr{
	1: {
		M.ParseAddr("149.154.175.10"),
		M.ParseAddr("2001:b28:f23d:f001::e"),
	},
	2: {
		M.ParseAddr("149.154.167.40"),
		M.ParseAddr("2001:67c:04e8:f002::e"),
	},
	3: {
		M.ParseAddr("149.154.175.117"),
		M.ParseAddr("2001:b28:f23d:f003::e"),
	},
}

func DataCenterName(dataCenter int) string {
	switch dataCenter {
	case 1:
		return "pluto"
	case 2:
		return "venus"
	case 3:
		return "aurora"
	case 4:
		return "vesta"
	case 5:
		return "flora"
	default:
		return F.ToString(dataCenter)
	}
}
