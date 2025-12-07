package settings

import "github.com/sagernet/sing-box/adapter"

type WIFIMonitor interface {
	ReadWIFIState() adapter.WIFIState
	Start() error
	Close() error
}
