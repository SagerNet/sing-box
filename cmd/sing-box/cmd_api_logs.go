package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	commandAPILogsFlagFollow bool
	commandAPILogsFlagLevel  string
	commandAPILogsFlagSearch string
)

var commandAPILogs = &cobra.Command{
	Use:   "logs",
	Short: "Print the service logs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPILogs()
	},
}

func init() {
	commandAPILogs.Flags().BoolVarP(&commandAPILogsFlagFollow, "follow", "f", false, "Keep printing new log entries until interrupted")
	commandAPILogs.Flags().StringVar(&commandAPILogsFlagLevel, "level", "", "Print entries at this level or more severe (default: the service log level)")
	commandAPILogs.Flags().StringVar(&commandAPILogsFlagSearch, "search", "", "Print entries containing this text, case-insensitive")
	commandAPIRoot.AddCommand(commandAPILogs)
}

func runAPILogs() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var level log.Level
	if commandAPILogsFlagLevel != "" {
		level, err = log.ParseLevel(commandAPILogsFlagLevel)
		if err != nil {
			return err
		}
	} else {
		defaultLevel, levelErr := client.GetDefaultLogLevel(ctx, &emptypb.Empty{})
		if levelErr != nil {
			return levelErr
		}
		level = log.Level(defaultLevel.GetLevel())
	}
	stream, err := client.SubscribeLog(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	searchQuery := strings.ToLower(strings.TrimSpace(commandAPILogsFlagSearch))
	for backlog := true; ; backlog = false {
		message, recvErr := stream.Recv()
		if recvErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return recvErr
		}
		if message.GetReset_() && len(message.GetMessages()) == 0 && !backlog {
			writeStderrLine("log buffer cleared")
			continue
		}
		var output strings.Builder
		for _, entry := range message.GetMessages() {
			if log.Level(entry.GetLevel()) > level {
				continue
			}
			plainMessage := stripColors(entry.GetMessage())
			if searchQuery != "" && !strings.Contains(strings.ToLower(plainMessage), searchQuery) {
				continue
			}
			if stdoutIsTerminal {
				output.WriteString(entry.GetMessage())
			} else {
				output.WriteString(plainMessage)
			}
			output.WriteString("\n")
		}
		os.Stdout.WriteString(output.String())
		if backlog && !commandAPILogsFlagFollow {
			return nil
		}
	}
}
