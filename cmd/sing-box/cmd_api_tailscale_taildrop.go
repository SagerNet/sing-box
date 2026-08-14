package main

import (
	"context"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/daemon"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandAPITailscaleTaildrop = &cobra.Command{
	Use:   "taildrop",
	Short: "Send and receive files over Tailscale",
}

func init() {
	commandAPITailscaleTaildrop.PersistentFlags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscaleTaildrop)
}

func fetchTaildropInbox(client daemon.StartedServiceClient, endpointTag string) (*daemon.TaildropInbox, error) {
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	stream, err := client.SubscribeTaildropInbox(ctx, &daemon.SubscribeTaildropInboxRequest{
		EndpointTag: endpointTag,
	})
	if err != nil {
		return nil, err
	}
	return stream.Recv()
}

func formatTaildropSize(size int64) string {
	switch {
	case size < 0:
		return "-"
	case size < 1000:
		return F.ToString(size, " B")
	case size < 1000*1000:
		return formatTaildropSizeUnit(size, 1000, "KB")
	case size < 1000*1000*1000:
		return formatTaildropSizeUnit(size, 1000*1000, "MB")
	default:
		return formatTaildropSizeUnit(size, 1000*1000*1000, "GB")
	}
}

func formatTaildropSizeUnit(size int64, unit int64, unitName string) string {
	value := float64(size) / float64(unit)
	var precision int
	switch {
	case value < 10:
		precision = 2
	case value < 100:
		precision = 1
	}
	return strconv.FormatFloat(value, 'f', precision, 64) + " " + unitName
}

func formatTaildropModifiedAt(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Local().Format("2006-01-02 15:04")
}
