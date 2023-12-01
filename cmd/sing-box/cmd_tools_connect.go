package main

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"

	"github.com/spf13/cobra"
)

var commandConnectFlagNetwork string

var commandConnect = &cobra.Command{
	Use:   "connect <address>",
	Short: "Connect to an address",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := connect(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandConnect.Flags().StringVarP(&commandConnectFlagNetwork, "network", "n", "tcp", "network type")
	commandTools.AddCommand(commandConnect)
}

func connect(address string) error {
	switch N.NetworkName(commandConnectFlagNetwork) {
	case N.NetworkTCP, N.NetworkUDP:
	default:
		return E.Cause(N.ErrUnknownNetwork, commandConnectFlagNetwork)
	}
	instance, err := createPreStartedClient()
	if err != nil {
		return err
	}
	defer instance.Close()
	dialer, err := createDialer(instance, commandConnectFlagNetwork, commandToolsFlagOutbound)
	if err != nil {
		return err
	}
	conn, err := dialer.DialContext(context.Background(), commandConnectFlagNetwork, M.ParseSocksaddr(address))
	if err != nil {
		return E.Cause(err, "connect to server")
	}
	var group task.Group
	group.Append("upload", func(ctx context.Context) error {
		return common.Error(bufio.Copy(conn, os.Stdin))
	})
	group.Append("download", func(ctx context.Context) error {
		return common.Error(bufio.Copy(os.Stdout, conn))
	})
	group.Cleanup(func() {
		conn.Close()
	})
	err = group.Run(context.Background())
	if E.IsClosed(err) {
		log.Info(err)
	} else {
		log.Error(err)
	}
	return nil
}
