package libbox

import (
	"context"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing/service/pause"
)

func NewBoxService(ctx context.Context, cancel context.CancelFunc, instance *box.Box, pauseManager pause.Manager, urlTestHistoryStorage *urltest.HistoryStorage) BoxService {
	return BoxService{
		ctx:                   ctx,
		cancel:                cancel,
		instance:              instance,
		pauseManager:          pauseManager,
		urlTestHistoryStorage: urlTestHistoryStorage,
	}
}

func (b *BoxService) GetInstance() *box.Box {
	return b.instance
}
