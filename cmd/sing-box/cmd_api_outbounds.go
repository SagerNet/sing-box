package main

import (
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIOutbounds = &cobra.Command{
	Use:   "outbounds",
	Short: "List outbounds",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIOutbounds()
	},
}

func init() {
	commandAPIRoot.AddCommand(commandAPIOutbounds)
}

func runAPIOutbounds() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	stream, err := client.SubscribeOutbounds(globalCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	outbounds, err := stream.Recv()
	if err != nil {
		return err
	}
	table := tableWriter{
		header:       []string{"TAG", "TYPE", "DELAY"},
		emptyMessage: "no outbounds",
	}
	for _, item := range outbounds.GetOutbounds() {
		table.addRow(item.GetTag(), item.GetType(), formatDelay(item.GetUrlTestDelay()))
	}
	table.flush()
	return nil
}

func formatDelay(delay int32) string {
	if delay <= 0 {
		return ""
	}
	return F.ToString(delay, " ms")
}
