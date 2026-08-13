package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIUsbipStatus = &cobra.Command{
	Use:   "status",
	Short: "Print the shared USB devices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIUsbipStatus()
	},
}

func init() {
	commandAPIUsbipStatus.Flags().StringVar(&commandAPIUsbipStatusFlagService, "service", "", "Restrict to one usbip-server tag")
	commandAPIUsbip.AddCommand(commandAPIUsbipStatus)
}

func runAPIUsbipStatus() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	servers, err := fetchUsbipServers(ctx, client)
	if err != nil {
		return err
	}
	if commandAPIUsbipStatusFlagService != "" {
		server, findErr := resolveUsbipServer(servers, commandAPIUsbipStatusFlagService)
		if findErr != nil {
			return findErr
		}
		servers = []*daemon.USBIPServerStatus{server}
	}
	for index, server := range servers {
		if len(servers) > 1 {
			if index > 0 {
				os.Stdout.WriteString("\n")
			}
			os.Stdout.WriteString(server.GetServerTag() + "\n")
		}
		table := tableWriter{
			header:       []string{"BUSID", "VID:PID", "PRODUCT", "STATE"},
			emptyMessage: "no shared devices",
		}
		for _, device := range server.GetDevices() {
			table.addRow(
				device.GetBusId(),
				usbipVendorProduct(uint16(device.GetDescriptor_().GetVendorId()), uint16(device.GetDescriptor_().GetProductId())),
				device.GetDescriptor_().GetProduct(),
				usbipDeviceStateString(device.GetState()),
			)
		}
		table.flush()
	}
	return nil
}

// SubscribeUSBIPServerStatus never completes: with no dynamic usbip-server it sends
// a single empty update and then blocks until the client cancels.
func fetchUsbipServers(ctx context.Context, client daemon.StartedServiceClient) ([]*daemon.USBIPServerStatus, error) {
	stream, err := client.SubscribeUSBIPServerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	update, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	return update.GetServers(), nil
}

func resolveUsbipServer(servers []*daemon.USBIPServerStatus, tag string) (*daemon.USBIPServerStatus, error) {
	if tag != "" {
		index := slices.IndexFunc(servers, func(it *daemon.USBIPServerStatus) bool {
			return it.GetServerTag() == tag
		})
		if index == -1 {
			return nil, E.New("usbip-server not found: ", tag, usbipServerTagHint(servers))
		}
		return servers[index], nil
	}
	switch len(servers) {
	case 0:
		return nil, E.New("no usbip-server found")
	case 1:
		return servers[0], nil
	default:
		return nil, E.New("multiple usbip-servers found, select one with --service", usbipServerTagHint(servers))
	}
}

func usbipServerTagHint(servers []*daemon.USBIPServerStatus) string {
	if len(servers) == 0 {
		return " (no usbip-server with dynamic provider found)"
	}
	return " (known tags: " + strings.Join(common.Map(servers, func(it *daemon.USBIPServerStatus) string {
		return it.GetServerTag()
	}), ", ") + ")"
}

func usbipVendorProduct(vendorID uint16, productID uint16) string {
	return fmt.Sprintf("%04x:%04x", vendorID, productID)
}

func usbipDeviceStateString(state daemon.USBDeviceState) string {
	return strings.ToLower(strings.TrimPrefix(state.String(), "USB_DEVICE_STATE_"))
}
