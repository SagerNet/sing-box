package mtproto

import (
	"math/rand"

	M "github.com/sagernet/sing/common/metadata"
)

const (
	// DefaultDC defines a number of the default DC to use. This value used
	// only if a value from obfuscated2 handshake frame is 0 (default).
	DefaultDC = 2
)

// https://github.com/telegramdesktop/tdesktop/blob/master/Telegram/SourceFiles/mtproto/mtproto_dc_options.cpp#L30
var (
	productionV4Addresses = [][]M.Socksaddr{
		{ // dc1
			M.ParseSocksaddr("149.154.175.50:443"),
		},
		{ // dc2
			M.ParseSocksaddr("149.154.167.51:443"),
			M.ParseSocksaddr("95.161.76.100:443"),
		},
		{ // dc3
			M.ParseSocksaddr("149.154.175.100:443"),
		},
		{ // dc4
			M.ParseSocksaddr("149.154.167.91:443"),
		},
		{ // dc5
			M.ParseSocksaddr("149.154.171.5:443"),
		},
	}
	productionV6Addresses = [][]M.Socksaddr{
		{ // dc1
			M.ParseSocksaddr("[2001:b28:f23d:f001::a]:443"),
		},
		{ // dc2
			M.ParseSocksaddr("[2001:67c:04e8:f002::a]:443"),
		},
		{ // dc3
			M.ParseSocksaddr("[2001:b28:f23d:f003::a]:443"),
		},
		{ // dc4
			M.ParseSocksaddr("[2001:67c:04e8:f004::a]:443"),
		},
		{ // dc5
			M.ParseSocksaddr("[2001:b28:f23f:f005::a]:443"),
		},
	}

	/*testV4Addresses = [][]M.Socksaddr{
		{ // dc1
			M.ParseSocksaddr("149.154.175.10:443"),
		},
		{ // dc2
			M.ParseSocksaddr("149.154.167.40:443"),
		},
		{ // dc3
			M.ParseSocksaddr("149.154.175.117:443"),
		},
	}
	testV6Addresses = [][]M.Socksaddr{
		{ // dc1
			M.ParseSocksaddr("[2001:b28:f23d:f001::e]:443"),
		},
		{ // dc2
			M.ParseSocksaddr("[2001:67c:04e8:f002::e]:443"),
		},
		{ // dc3
			M.ParseSocksaddr("[2001:b28:f23d:f003::e]:443"),
		},
	}*/
)

type addressPool struct {
	v4 [][]M.Socksaddr
	v6 [][]M.Socksaddr
}

var AddressPool = addressPool{productionV4Addresses, productionV6Addresses}

func (a addressPool) IsValidDC(dc int) bool {
	return dc > 0 && dc <= len(a.v4) && dc <= len(a.v6)
}

func (a addressPool) getRandomDC() int {
	return 1 + rand.Intn(len(a.v4))
}

func (a addressPool) GetV4(dc int) []M.Socksaddr {
	return a.get(a.v4, dc-1)
}

func (a addressPool) GetV6(dc int) []M.Socksaddr {
	return a.get(a.v6, dc-1)
}

func (a addressPool) get(addresses [][]M.Socksaddr, dc int) []M.Socksaddr {
	if dc < 0 || dc >= len(addresses) {
		return nil
	}

	rv := make([]M.Socksaddr, len(addresses[dc]))
	copy(rv, addresses[dc])

	if len(rv) > 1 {
		rand.Shuffle(len(rv), func(i, j int) {
			rv[i], rv[j] = rv[j], rv[i]
		})
	}

	return rv
}
