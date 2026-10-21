package netns

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
)

var _ adapter.NetworkNamespaceManager = (*Manager)(nil)

func Hold() {
	signal.Ignore(os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	buffer := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buffer)
		if err != nil {
			os.Exit(0)
		}
	}
}
