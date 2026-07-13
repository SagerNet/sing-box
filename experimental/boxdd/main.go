package main

import (
	"context"
	"fmt"
	"os"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"

	"github.com/spf13/cobra"
)

const serviceName = "sing-box-daemon"

var mainCommand = &cobra.Command{
	Use:     serviceName,
	Version: C.Version,
}

var commandVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the daemon version",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		fmt.Println("sing-box-daemon version", C.Version)
		fmt.Println("core api version", daemon.APIVersion)
	},
}

func init() {
	mainCommand.AddCommand(commandVersion)
}

func main() {
	log.SetStdLogger(log.NewDefaultFactory(context.Background(), log.Formatter{
		BaseTime:      time.Now(),
		DisableColors: true,
	}, os.Stderr, "", nil, false).Logger())
	err := mainCommand.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
