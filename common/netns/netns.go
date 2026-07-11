package netns

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
)

var _ adapter.NetworkNamespaceManager = (*Manager)(nil)

func Hold() {
	buffer := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buffer)
		if err != nil {
			os.Exit(0)
		}
	}
}
