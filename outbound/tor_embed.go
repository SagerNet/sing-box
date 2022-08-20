//go:build with_embedded_tor

package outbound

import (
	"berty.tech/go-libtor"
	"github.com/cretz/bine/tor"
)

func newConfig() tor.StartConf {
	return tor.StartConf{
		ProcessCreator:         libtor.Creator,
		UseEmbeddedControlConn: true,
	}
}
