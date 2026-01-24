package main

import (
	"context"
	"os"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"

	"github.com/spf13/cobra"
)

var (
	commandSyncTimeFlagServer   string
	commandSyncTimeOutputFormat string
	commandSyncTimeWrite        bool
)

var commandSyncTime = &cobra.Command{
	Use:   "synctime",
	Short: "Sync time using the NTP protocol",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		err := syncTime()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandSyncTime.Flags().StringVarP(&commandSyncTimeFlagServer, "server", "s", "time.apple.com", "Set NTP server")
	commandSyncTime.Flags().StringVarP(&commandSyncTimeOutputFormat, "format", "f", C.TimeLayout, "Set output format")
	commandSyncTime.Flags().BoolVarP(&commandSyncTimeWrite, "write", "w", false, "Write time to system")
	commandTools.AddCommand(commandSyncTime)
}

func syncTime() error {
	instance, err := createPreStartedClient()
	if err != nil {
		return err
	}
	dialer, err := createDialer(instance, commandToolsFlagOutbound)
	if err != nil {
		return err
	}
	defer instance.Close()
	serverAddress := M.ParseSocksaddr(commandSyncTimeFlagServer)
	if serverAddress.Port == 0 {
		serverAddress.Port = 123
	}
	response, err := ntp.Exchange(context.Background(), dialer, serverAddress)
	if err != nil {
		return err
	}
	if commandSyncTimeWrite {
		err = ntp.SetSystemTime(response.Time)
		if err != nil {
			return E.Cause(err, "write time to system")
		}
	}
	os.Stdout.WriteString(response.Time.Local().Format(commandSyncTimeOutputFormat))
	return nil
}
