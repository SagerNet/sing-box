package daemon

import (
	"bytes"
	"context"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

type Instance struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	instance              *box.Box
	clashServer           adapter.ClashServer
	cacheFile             adapter.CacheFile
	pauseManager          pause.Manager
	urlTestHistoryStorage *urltest.HistoryStorage
}

func (s *StartedService) CheckConfig(configContent string) error {
	options, err := parseConfig(s.ctx, configContent)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err == nil {
		instance.Close()
	}
	return err
}

func (s *StartedService) FormatConfig(configContent string) (string, error) {
	options, err := parseConfig(s.ctx, configContent)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(options)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

type OverrideOptions struct {
	AutoRedirect   bool
	IncludePackage []string
	ExcludePackage []string
}

func (s *StartedService) newInstance(profileContent string, overrideOptions *OverrideOptions) (*Instance, error) {
	ctx := s.ctx
	service.MustRegister[deprecated.Manager](ctx, new(deprecatedManager))
	ctx, cancel := context.WithCancel(include.Context(ctx))
	options, err := parseConfig(ctx, profileContent)
	if err != nil {
		cancel()
		return nil, err
	}
	if overrideOptions != nil {
		for _, inbound := range options.Inbounds {
			if tunInboundOptions, isTUN := inbound.Options.(*option.TunInboundOptions); isTUN {
				tunInboundOptions.AutoRedirect = overrideOptions.AutoRedirect
				tunInboundOptions.IncludePackage = append(tunInboundOptions.IncludePackage, overrideOptions.IncludePackage...)
				tunInboundOptions.ExcludePackage = append(tunInboundOptions.ExcludePackage, overrideOptions.ExcludePackage...)
				break
			}
		}
	}
	urlTestHistoryStorage := urltest.NewHistoryStorage()
	ctx = service.ContextWithPtr(ctx, urlTestHistoryStorage)
	i := &Instance{
		ctx:                   ctx,
		cancel:                cancel,
		urlTestHistoryStorage: urlTestHistoryStorage,
	}
	boxInstance, err := box.New(box.Options{
		Context:           ctx,
		Options:           options,
		PlatformLogWriter: s,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	i.instance = boxInstance
	i.clashServer = service.FromContext[adapter.ClashServer](ctx)
	i.pauseManager = service.FromContext[pause.Manager](ctx)
	i.cacheFile = service.FromContext[adapter.CacheFile](ctx)
	return i, nil
}

func (i *Instance) Start() error {
	return i.instance.Start()
}

func (i *Instance) Close() error {
	i.cancel()
	i.urlTestHistoryStorage.Close()
	return i.instance.Close()
}

func (i *Instance) Box() *box.Box {
	return i.instance
}

func (i *Instance) PauseManager() pause.Manager {
	return i.pauseManager
}

func parseConfig(ctx context.Context, configContent string) (option.Options, error) {
	options, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(configContent))
	if err != nil {
		return option.Options{}, E.Cause(err, "decode config")
	}
	return options, nil
}
