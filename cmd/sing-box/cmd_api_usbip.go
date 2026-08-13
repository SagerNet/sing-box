package main

import (
	"github.com/spf13/cobra"
)

var (
	commandAPIUsbipStatusFlagService string
	commandAPIUsbipShareFlagService  string
	commandAPIUsbipShareFlagAll      bool
	commandAPIUsbipShareFlagCapture  bool
)

var commandAPIUsbip = &cobra.Command{
	Use:   "usbip",
	Short: "Manage USB/IP device sharing",
}

var commandAPIUsbipDevice = &cobra.Command{
	Use:   "device",
	Short: "Manage local USB devices",
}

var commandAPIUsbipDeviceList = &cobra.Command{
	Use:   "list",
	Short: "List local USB devices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIUsbipDeviceList()
	},
}

var commandAPIUsbipDeviceShow = &cobra.Command{
	Use:   "show <device>",
	Short: "Print a local USB device",
	Long: "Print a local USB device.\n\n" +
		"The device is selected by bus id, by vid:pid, or by vid:pid:serial.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIUsbipDeviceShow(args[0])
	},
}

var commandAPIUsbipShare = &cobra.Command{
	Use:   "share <device>...",
	Short: "Share local USB devices through a usbip-server",
	Long: "Share local USB devices through a usbip-server.\n\n" +
		"Each device is selected by bus id, by vid:pid, or by vid:pid:serial.\n" +
		"Sharing lasts until the command is interrupted.",
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIUsbipShare(args)
	},
}

func init() {
	commandAPIUsbipShare.Flags().StringVar(&commandAPIUsbipShareFlagService, "service", "", "usbip-server tag (default: the only usbip-server with dynamic provider)")
	commandAPIUsbipShare.Flags().BoolVar(&commandAPIUsbipShareFlagAll, "all", false, "Share every local USB device, including newly plugged ones")
	commandAPIUsbipShare.Flags().BoolVar(&commandAPIUsbipShareFlagCapture, "capture", false, "Detach devices from their current driver before sharing (requires root or Administrator)")
	commandAPIUsbipDevice.AddCommand(commandAPIUsbipDeviceList)
	commandAPIUsbipDevice.AddCommand(commandAPIUsbipDeviceShow)
	commandAPIUsbip.AddCommand(commandAPIUsbipDevice)
	commandAPIUsbip.AddCommand(commandAPIUsbipShare)
	commandAPIRoot.AddCommand(commandAPIUsbip)
}
