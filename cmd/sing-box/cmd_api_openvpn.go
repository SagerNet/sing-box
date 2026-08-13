package main

import (
	"context"
	"errors"
	"io"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	openVPNChallengeCredentials = "credentials"
	openVPNChallengeSecret      = "secret"
	openVPNChallengeMessage     = "message"
	openVPNChallengeOpenURL     = "open-url"
)

var commandAPIOpenVPN = &cobra.Command{
	Use:   "openvpn",
	Short: "Manage OpenVPN authentication",
}

func init() {
	commandAPIRoot.AddCommand(commandAPIOpenVPN)
}

func subscribeOpenVPNStatus(ctx context.Context, client daemon.StartedServiceClient) (grpc.ServerStreamingClient[daemon.OpenVPNStatusUpdate], []*daemon.OpenVPNEndpointStatus, error) {
	stream, err := client.SubscribeOpenVPNStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, nil, err
	}
	endpoints, err := recvOpenVPNStatus(stream)
	if err != nil {
		return nil, nil, err
	}
	return stream, endpoints, nil
}

func recvOpenVPNStatus(stream grpc.ServerStreamingClient[daemon.OpenVPNStatusUpdate]) ([]*daemon.OpenVPNEndpointStatus, error) {
	update, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, E.New("api service closed the status stream")
		}
		return nil, err
	}
	return update.GetEndpoints(), nil
}

func openVPNChallengeSummary(challenge *daemon.OpenVPNChallenge) string {
	switch challenge.GetKind() {
	case openVPNChallengeMessage, openVPNChallengeOpenURL:
		return challenge.GetKind() + " (not answerable)"
	default:
		return challenge.GetKind()
	}
}
