//go:build with_subscribe

package main

import (
	"context"
	"encoding/json"
	"fmt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/subscribe"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/spf13/cobra"
	"time"
)

var commandShowSubscribePeer = &cobra.Command{
	Use:   "showsub",
	Short: "Show subscribe peer",
	Run: func(cmd *cobra.Command, args []string) {
		err := showSubscribePeer()
		if err != nil {
			log.Fatal(err)
		}
	},
}

var showSubscribePeerTags []string

func init() {
	commandShowSubscribePeer.Flags().StringArrayVarP(&showSubscribePeerTags, "tag", "t", nil, "set tag")
	mainCommand.AddCommand(commandShowSubscribePeer)
}

func showSubscribePeer() error {
	options, err := readConfigAndMerge()
	if err != nil {
		return err
	}

	if options.Outbounds == nil || len(options.Outbounds) == 0 {
		return E.New("no outbound found")
	}

	subscribeOptionsMap := make(map[string]option.Outbound)

	all := false

	if showSubscribePeerTags == nil || len(showSubscribePeerTags) == 0 {
		all = true
	} else {
		for _, t := range showSubscribePeerTags {
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

	outs := make([]option.Outbound, 0)

	for _, o := range subscribeOptionsMap {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		peers, err := subscribe.ParsePeer(ctx, o.Tag, o.SubscribeOptions)
		cancel()
		if err != nil {
			return E.Cause(err, "show subscribe '", o.Tag, "' peer fail")
		}
		outs = append(outs, peers...)
	}

	m := map[string]any{
		"outbounds": outs,
	}

	content, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return E.Cause(err, "show subscribe peer fail")
	}

	fmt.Println(string(content))

	return nil
}
