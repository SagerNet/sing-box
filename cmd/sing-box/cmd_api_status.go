package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common/byteformats"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIStatus = &cobra.Command{
	Use:   "status",
	Short: "Print the service status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPIStatus()
	},
}

func init() {
	commandAPIRoot.AddCommand(commandAPIStatus)
}

func runAPIStatus() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	statusStream, err := client.SubscribeStatus(ctx, &daemon.SubscribeStatusRequest{Interval: int64(time.Second)})
	if err != nil {
		return err
	}
	var (
		waitGroup     sync.WaitGroup
		serviceStatus *daemon.ServiceStatus
		startedAt     *daemon.StartedAt
	)
	waitGroup.Go(func() {
		serviceStatusStream, statusErr := client.SubscribeServiceStatus(ctx, &emptypb.Empty{})
		if statusErr != nil {
			return
		}
		serviceStatus, _ = serviceStatusStream.Recv()
	})
	waitGroup.Go(func() {
		startedAt, _ = client.GetStartedAt(ctx, &emptypb.Empty{})
	})
	status, err := statusStream.Recv()
	if err != nil {
		return err
	}
	rateStatus, err := statusStream.Recv()
	if err == nil {
		status = rateStatus
	}
	waitGroup.Wait()

	var state string
	if serviceStatus != nil {
		state = strings.ToLower(serviceStatus.GetStatus().String())
	}
	var uptime string
	if startedAt.GetStartedAt() > 0 {
		uptime = time.Since(time.UnixMilli(startedAt.GetStartedAt())).Truncate(time.Second).String()
	}
	var connections string
	if status.GetTrafficAvailable() {
		connections = F.ToString(status.GetConnectionsIn(), " in / ", status.GetConnectionsOut(), " out")
	} else {
		connections = F.ToString("- in / ", status.GetConnectionsOut(), " out")
	}
	var uplink, downlink string
	if status.GetTrafficAvailable() {
		uplink = F.ToString(byteformats.FormatBytes(uint64(status.GetUplink())), "/s (", byteformats.FormatBytes(uint64(status.GetUplinkTotal())), " total)")
		downlink = F.ToString(byteformats.FormatBytes(uint64(status.GetDownlink())), "/s (", byteformats.FormatBytes(uint64(status.GetDownlinkTotal())), " total)")
	}
	var block blockWriter
	block.addLine("State", state)
	block.addLine("Uptime", uptime)
	block.addLine("Memory", byteformats.FormatMemoryBytes(status.GetMemory()))
	block.addLine("Goroutines", F.ToString(status.GetGoroutines()))
	block.addLine("Connections", connections)
	block.addLine("Uplink", uplink)
	block.addLine("Downlink", downlink)
	if serviceStatus.GetStatus() == daemon.ServiceStatus_FATAL {
		block.addLine("Error", serviceStatus.GetErrorMessage())
	}
	block.flush()
	return nil
}
