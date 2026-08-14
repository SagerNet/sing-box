package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

const (
	commandAPITailscalePingCount   = 10
	commandAPITailscalePingTimeout = 5 * time.Second
)

var commandAPITailscalePing = &cobra.Command{
	Use:   "ping <peer>",
	Short: "Ping a Tailscale peer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITailscalePing(args[0])
	},
}

func init() {
	commandAPITailscalePing.Flags().StringVar(&commandAPITailscaleFlagEndpoint, "endpoint", "", commandAPITailscaleEndpointUsage)
	commandAPITailscale.AddCommand(commandAPITailscalePing)
}

func runAPITailscalePing(selector string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	entry, err := resolveTailscalePeer(tailscalePeerEntries(endpoint), selector)
	if err != nil {
		return err
	}
	peerAddress := tailscalePeerAddress(entry.peer)
	if peerAddress == "" {
		return E.New("peer has no tailscale address: ", tailscalePeerName(entry.peer))
	}
	peerName, _, _ := strings.Cut(tailscalePeerName(entry.peer), ".")
	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stream, err := client.StartTailscalePing(ctx, &daemon.TailscalePingRequest{
		EndpointTag: endpoint.GetEndpointTag(),
		PeerIP:      peerAddress,
	})
	if err != nil {
		return err
	}
	responses := make(chan *daemon.TailscalePingResponse)
	streamErrors := make(chan error, 1)
	go func() {
		for {
			pingResponse, pingErr := stream.Recv()
			if pingErr != nil {
				streamErrors <- pingErr
				return
			}
			select {
			case responses <- pingResponse:
			case <-ctx.Done():
				return
			}
		}
	}()
	timer := time.NewTimer(commandAPITailscalePingTimeout)
	defer timer.Stop()
	var (
		pongCount     int
		lastPeerRelay string
	)
	for pongCount < commandAPITailscalePingCount {
		var (
			response *daemon.TailscalePingResponse
			recvErr  error
		)
		select {
		case response = <-responses:
		case recvErr = <-streamErrors:
		case <-timer.C:
			return E.New("no reply from ", peerName, " (", peerAddress, ") after ", commandAPITailscalePingTimeout.String())
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			if pongCount > 0 {
				return nil
			}
			return E.New("interrupted")
		}
		if recvErr != nil {
			return recvErr
		}
		if response.GetError() != "" {
			return E.New("ping error: ", response.GetError())
		}
		os.Stdout.WriteString(formatTailscalePong(peerName, peerAddress, response) + "\n")
		pongCount++
		if response.GetEndpoint() != "" {
			return nil
		}
		lastPeerRelay = response.GetPeerRelay()
		timer.Reset(commandAPITailscalePingTimeout)
	}
	if lastPeerRelay != "" {
		os.Stdout.WriteString(F.ToString("direct connection not established, relayed by peer relay ", lastPeerRelay, "\n"))
	} else {
		os.Stdout.WriteString("direct connection not established\n")
	}
	return nil
}

func formatTailscalePong(peerName string, peerAddress string, response *daemon.TailscalePingResponse) string {
	via := response.GetEndpoint()
	if via == "" {
		if response.GetPeerRelay() != "" {
			via = F.ToString("peer relay ", response.GetPeerRelay())
		} else if response.GetDerpRegionCode() != "" {
			via = F.ToString("DERP(", response.GetDerpRegionCode(), ")")
		} else {
			via = F.ToString("DERP(", response.GetDerpRegionID(), ")")
		}
	}
	latency := time.Duration(response.GetLatencyMs() * float64(time.Millisecond))
	rounded := latency.Round(time.Millisecond)
	if rounded == 0 {
		rounded = latency.Round(time.Microsecond)
	}
	return F.ToString("pong from ", peerName, " (", peerAddress, ") via ", via, " in ", rounded.String())
}
