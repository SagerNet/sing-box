package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/common/networkquality"
	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	commandAPINetworkQualityFlagConfigURL  string
	commandAPINetworkQualityFlagSerial     bool
	commandAPINetworkQualityFlagMaxRuntime int
	commandAPINetworkQualityFlagHTTP3      bool
	commandAPINetworkQualityFlagOutbound   string
)

var commandAPINetworkQuality = &cobra.Command{
	Use:   "networkquality",
	Short: "Run a network quality test",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPINetworkQuality()
	},
}

func init() {
	commandAPINetworkQuality.Flags().StringVar(
		&commandAPINetworkQualityFlagConfigURL,
		"config-url", "",
		"Network quality test config URL (default: Apple mensura)",
	)
	commandAPINetworkQuality.Flags().BoolVar(
		&commandAPINetworkQualityFlagSerial,
		"serial", false,
		"Run download and upload tests sequentially instead of in parallel",
	)
	commandAPINetworkQuality.Flags().IntVar(
		&commandAPINetworkQualityFlagMaxRuntime,
		"max-runtime", int(networkquality.DefaultMaxRuntime/time.Second),
		"Network quality maximum runtime in seconds",
	)
	commandAPINetworkQuality.Flags().BoolVar(
		&commandAPINetworkQualityFlagHTTP3,
		"http3", false,
		"Use HTTP/3 (QUIC) for measurement traffic",
	)
	commandAPINetworkQuality.Flags().StringVarP(
		&commandAPINetworkQualityFlagOutbound,
		"outbound", "o", "",
		"Use specified tag instead of default outbound",
	)
	commandAPIRoot.AddCommand(commandAPINetworkQuality)
}

func runAPINetworkQuality() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	stream, err := client.StartNetworkQualityTest(globalCtx, &daemon.NetworkQualityTestRequest{
		ConfigURL:         commandAPINetworkQualityFlagConfigURL,
		OutboundTag:       commandAPINetworkQualityFlagOutbound,
		Serial:            commandAPINetworkQualityFlagSerial,
		MaxRuntimeSeconds: int32(commandAPINetworkQualityFlagMaxRuntime),
		Http3:             commandAPINetworkQualityFlagHTTP3,
	})
	if err != nil {
		return err
	}
	writeStderrLine("==== NETWORK QUALITY TEST ====")
	for {
		progress, recvErr := stream.Recv()
		if recvErr != nil {
			return recvErr
		}
		if !progress.GetIsFinal() {
			writeNetworkQualityProgress(progress)
			continue
		}
		writeStderrLine("")
		if progress.GetError() != "" {
			return E.New(progress.GetError())
		}
		writeStderrLine(strings.Repeat("-", 40))
		fmt.Fprintf(os.Stdout, "Idle Latency:            %d ms\n", progress.GetIdleLatencyMs())
		fmt.Fprintf(os.Stdout, "Download Capacity:       %-20s Accuracy: %s\n",
			networkquality.FormatBitrate(progress.GetDownloadCapacity()),
			networkquality.Accuracy(progress.GetDownloadCapacityAccuracy()))
		fmt.Fprintf(os.Stdout, "Upload Capacity:         %-20s Accuracy: %s\n",
			networkquality.FormatBitrate(progress.GetUploadCapacity()),
			networkquality.Accuracy(progress.GetUploadCapacityAccuracy()))
		fmt.Fprintf(os.Stdout, "Download Responsiveness: %-20s Accuracy: %s\n",
			fmt.Sprintf("%d RPM", progress.GetDownloadRPM()),
			networkquality.Accuracy(progress.GetDownloadRPMAccuracy()))
		fmt.Fprintf(os.Stdout, "Upload Responsiveness:   %-20s Accuracy: %s\n",
			fmt.Sprintf("%d RPM", progress.GetUploadRPM()),
			networkquality.Accuracy(progress.GetUploadRPMAccuracy()))
		return nil
	}
}

func writeNetworkQualityProgress(progress *daemon.NetworkQualityTestProgress) {
	if !commandAPINetworkQualityFlagSerial && networkquality.Phase(progress.GetPhase()) != networkquality.PhaseIdle {
		writeProgress(fmt.Sprintf("Download: %s  RPM: %d  Upload: %s  RPM: %d",
			networkquality.FormatBitrate(progress.GetDownloadCapacity()), progress.GetDownloadRPM(),
			networkquality.FormatBitrate(progress.GetUploadCapacity()), progress.GetUploadRPM()))
		return
	}
	switch networkquality.Phase(progress.GetPhase()) {
	case networkquality.PhaseIdle:
		if progress.GetIdleLatencyMs() > 0 {
			writeProgress(fmt.Sprintf("Idle Latency: %d ms", progress.GetIdleLatencyMs()))
		} else {
			writeProgress("Measuring idle latency...")
		}
	case networkquality.PhaseDownload:
		writeProgress(fmt.Sprintf("Download: %s  RPM: %d",
			networkquality.FormatBitrate(progress.GetDownloadCapacity()), progress.GetDownloadRPM()))
	case networkquality.PhaseUpload:
		writeProgress(fmt.Sprintf("Upload: %s  RPM: %d",
			networkquality.FormatBitrate(progress.GetUploadCapacity()), progress.GetUploadRPM()))
	}
}
