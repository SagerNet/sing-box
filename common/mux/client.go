package mux

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-mux"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type Client = mux.Client

func NewClientWithOptions(dialer N.Dialer, logger logger.Logger, options option.OutboundMultiplexOptions) (*Client, error) {
	if !options.Enabled {
		return nil, nil
	}
	var brutalOptions mux.BrutalOptions
	if options.Brutal != nil && options.Brutal.Enabled {
		brutalOptions = mux.BrutalOptions{
			Enabled:    true,
			SendBPS:    uint64(options.Brutal.UpMbps * C.MbpsToBps),
			ReceiveBPS: uint64(options.Brutal.DownMbps * C.MbpsToBps),
		}
		if brutalOptions.SendBPS < mux.BrutalMinSpeedBPS {
			return nil, E.New("brutal: invalid upload speed")
		}
		if brutalOptions.ReceiveBPS < mux.BrutalMinSpeedBPS {
			return nil, E.New("brutal: invalid download speed")
		}
	}
	return mux.NewClient(mux.Options{
		Dialer:         dialer,
		Logger:         logger,
		Protocol:       options.Protocol,
		MaxConnections: options.MaxConnections,
		MinStreams:     options.MinStreams,
		MaxStreams:     options.MaxStreams,
		Padding:        options.Padding,
		Brutal:         brutalOptions,
	})
}
