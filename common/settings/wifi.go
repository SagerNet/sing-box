package settings

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
)

type WIFIMonitor interface {
	ReadWIFIState(ctx context.Context) adapter.WIFIState
	Start() error
	Close() error
}
