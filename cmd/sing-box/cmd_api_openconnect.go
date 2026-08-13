package main

import (
	"context"
	"errors"
	"io"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var commandAPIOpenConnect = &cobra.Command{
	Use:   "openconnect",
	Short: "Manage OpenConnect authentication",
}

func init() {
	commandAPIRoot.AddCommand(commandAPIOpenConnect)
}

func subscribeOpenConnectStatus(ctx context.Context, client daemon.StartedServiceClient) (grpc.ServerStreamingClient[daemon.OpenConnectStatusUpdate], []*daemon.OpenConnectEndpointStatus, error) {
	stream, err := client.SubscribeOpenConnectStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, nil, err
	}
	endpoints, err := recvOpenConnectStatus(stream)
	if err != nil {
		return nil, nil, err
	}
	return stream, endpoints, nil
}

func recvOpenConnectStatus(stream grpc.ServerStreamingClient[daemon.OpenConnectStatusUpdate]) ([]*daemon.OpenConnectEndpointStatus, error) {
	update, err := stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, E.New("api service closed the status stream")
		}
		return nil, err
	}
	return update.GetEndpoints(), nil
}

func openConnectChallengeSummary(challenge *daemon.OpenConnectAuthChallenge) string {
	form := challenge.GetForm()
	if form != nil {
		return F.ToString("form (", len(form.GetFields()), " fields)")
	}
	browser := challenge.GetBrowser()
	if browser == nil {
		return "unknown"
	}
	mode, err := deriveOpenConnectBrowserMode(browser)
	if err != nil {
		return "browser (invalid)"
	}
	return "browser (" + mode + ")"
}
