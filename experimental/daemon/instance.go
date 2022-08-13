package daemon

import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
)

type Instance struct {
	access      sync.Mutex
	boxInstance *box.Box
	boxCancel   context.CancelFunc
}

func (i *Instance) Running() bool {
	i.access.Lock()
	defer i.access.Unlock()
	return i.boxInstance != nil
}

func (i *Instance) Start(options option.Options) error {
	i.access.Lock()
	defer i.access.Unlock()
	if i.boxInstance != nil {
		i.boxCancel()
		i.boxInstance.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := box.New(ctx, options)
	if err != nil {
		cancel()
		return err
	}
	err = instance.Start()
	if err != nil {
		cancel()
		return err
	}
	i.boxInstance = instance
	i.boxCancel = cancel
	return nil
}

func (i *Instance) Close() error {
	i.access.Lock()
	defer i.access.Unlock()
	if i.boxInstance == nil {
		return os.ErrClosed
	}
	i.boxCancel()
	err := i.boxInstance.Close()
	i.boxInstance = nil
	i.boxCancel = nil
	return err
}
