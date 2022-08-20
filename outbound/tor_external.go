//go:build !with_embedded_tor

package outbound

import "github.com/cretz/bine/tor"

func newConfig() tor.StartConf {
	return tor.StartConf{}
}
