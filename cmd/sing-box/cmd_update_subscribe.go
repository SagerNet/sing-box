//go:build with_subscribe

package main

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/subscribe"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"time"
)

var commandUpdateSubscribe = &cobra.Command{
	Use:   "upsub",
	Short: "Update subscribe",
	Run: func(cmd *cobra.Command, args []string) {
		err := updateSubscribe()
		if err != nil {
			log.Fatal(err)
		}
	},
}

var updateSubscribeTags []string

func init() {
	commandUpdateSubscribe.Flags().StringArrayVarP(&updateSubscribeTags, "tag", "t", nil, "set tag")
	mainCommand.AddCommand(commandUpdateSubscribe)
}

func updateSubscribe() error {
	options, err := readConfigAndMerge()
	if err != nil {
		return err
	}

	if options.Outbounds == nil || len(options.Outbounds) == 0 {
		return E.New("no outbound found")
	}

	subscribeOptionsMap := make(map[string]option.Outbound)

	all := false

	if updateSubscribeTags == nil || len(updateSubscribeTags) == 0 {
		all = true
	} else {
		for _, t := range updateSubscribeTags {
			subscribeOptionsMap[t] = option.Outbound{}
		}
	}

	for _, o := range options.Outbounds {
		if all {
			if o.Type == C.TypeSubscribe {
				subscribeOptionsMap[o.Tag] = o
			}
			continue
		}

		if _, ok := subscribeOptionsMap[o.Tag]; ok {
			if o.Type != C.TypeSubscribe {
				return E.New("outbound ", o.Tag, " is not subscribe")
			}
			subscribeOptionsMap[o.Tag] = o
		}
	}

	if len(subscribeOptionsMap) > 0 {
		for tag, opt := range subscribeOptionsMap {
			if opt.Type == "" {
				return E.New("outbound ", tag, " not found")
			}
		}
	}

	for _, o := range subscribeOptionsMap {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := subscribe.RequestAndCache(ctx, o.SubscribeOptions)
		cancel()
		if err != nil {
			return E.Cause(err, "update subscribe '", o.Tag, "' fail")
		}
		log.Info("update subscribe '", o.Tag, "' success")
	}

	return nil
}
