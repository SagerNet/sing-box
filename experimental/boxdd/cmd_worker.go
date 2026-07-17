package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	workerSocketPath            string
	workerDaemonRelaySocketPath string
	workerParentProcessID       uint32
)

type workerParent interface {
	Close() error
}

var commandWorker = &cobra.Command{
	Use:   "worker",
	Short: "Serve the non-privileged application worker for the application process",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := runWorker()
		if err != nil {
			log.Fatal(E.Cause(err, "run application worker"))
		}
	},
}

func init() {
	commandWorker.Flags().StringVar(&workerSocketPath, "socket", "", "listen on the specified unix domain socket path, or named pipe path on Windows")
	commandWorker.Flags().StringVar(&workerDaemonRelaySocketPath, "daemon-relay-socket", "", "relay the authenticated Windows daemon connection on the specified named pipe path")
	commandWorker.Flags().Uint32Var(&workerParentProcessID, "parent-pid", 0, "expected application parent process ID")
	mainCommand.AddCommand(commandWorker)
}

func runWorker() error {
	if workerSocketPath == "" {
		return E.New("missing --socket")
	}
	if workerParentProcessID == 0 {
		return E.New("missing --parent-pid")
	}
	parent, err := prepareWorkerParent(workerParentProcessID)
	if err != nil {
		return err
	}
	defer parent.Close()
	listener, err := listenWorkerEndpoint(workerSocketPath, parent)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(daemon.UnaryLocaleInterceptor),
		grpc.ChainStreamInterceptor(daemon.StreamLocaleInterceptor),
	)
	RegisterApplicationServiceServer(server, &applicationService{
		startedService: daemon.NewStartedService(daemon.ServiceOptions{Context: include.Context(context.Background())}),
	})
	relayErrorChannel := make(chan error, 1)
	relay, err := startWorkerDaemonRelay(workerDaemonRelaySocketPath, parent, func(relayError error) {
		select {
		case relayErrorChannel <- relayError:
		default:
		}
		server.Stop()
	})
	if err != nil {
		return err
	}
	if relay != nil {
		defer relay.Close()
	}
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		if relay != nil {
			relay.Close()
		}
		server.Stop()
	}()
	fmt.Println("READY")
	err = server.Serve(listener)
	select {
	case relayError := <-relayErrorChannel:
		return relayError
	default:
	}
	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}
