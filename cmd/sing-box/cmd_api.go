package main

import (
	"os"
	"strings"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	commandAPIFlagURL    string
	commandAPIFlagSecret string
	commandAPIServerURL  string
)

var commandAPI = &cobra.Command{
	Use:                "api <command>",
	Short:              "API service client",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		err := runAPI(args)
		if err != nil {
			log.Fatal(err)
		}
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		targetCommand, remainingArgs, err := commandAPIRoot.Find(args)
		if err != nil || len(remainingArgs) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return common.Map(common.Filter(targetCommand.Commands(), func(it *cobra.Command) bool {
			return it.IsAvailableCommand() && strings.HasPrefix(it.Name(), toComplete)
		}), func(it *cobra.Command) string {
			return it.Name() + "\t" + it.Short
		}), cobra.ShellCompDirectiveNoFileComp
	},
}

var commandAPIRoot = &cobra.Command{
	Use:               "api",
	Short:             "API service client",
	SilenceUsage:      true,
	SilenceErrors:     true,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

func init() {
	commandAPIRoot.PersistentFlags().StringVar(&commandAPIFlagURL, "url", "", "API service URL (default: $BOX_API_URL)")
	commandAPIRoot.PersistentFlags().StringVar(&commandAPIFlagSecret, "secret", "", "API service secret (default: $BOX_API_SECRET)")
	mainCommand.AddCommand(commandAPI)
}

func runAPI(args []string) error {
	commandAPIRoot.SetArgs(append([]string{}, args...))
	err := commandAPIRoot.Execute()
	if err == nil {
		return nil
	}
	grpcStatus, isStatus := status.FromError(err)
	if !isStatus {
		return err
	}
	switch grpcStatus.Code() {
	case codes.Unavailable:
		return E.New("failed to connect to API service at ", commandAPIServerURL, ": ", grpcStatus.Message())
	case codes.Unknown:
		return E.New(grpcStatus.Message())
	case codes.Unimplemented:
		return E.New(grpcStatus.Code().String(), ": ", grpcStatus.Message(), " (client API version ", daemon.APIVersion, ")")
	default:
		return E.New(grpcStatus.Code().String(), ": ", grpcStatus.Message())
	}
}

func createAPIClient() (*grpc.ClientConn, daemon.StartedServiceClient, error) {
	serverURL := commandAPIFlagURL
	if serverURL == "" {
		serverURL = os.Getenv("BOX_API_URL")
	}
	if serverURL == "" {
		return nil, nil, E.New("missing API service URL, set --url or BOX_API_URL")
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	commandAPIServerURL = serverURL
	secret := commandAPIFlagSecret
	if secret == "" {
		secret = os.Getenv("BOX_API_SECRET")
	}
	clientConn, err := daemon.NewRemoteClient(daemon.RemoteClientOptions{
		ServerURL: serverURL,
		Secret:    secret,
	})
	if err != nil {
		return nil, nil, err
	}
	return clientConn, daemon.NewStartedServiceClient(clientConn), nil
}
