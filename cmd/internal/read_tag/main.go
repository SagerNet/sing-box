package main

import (
	"os"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
)

func main() {
	currentTag, err := build_shared.ReadTag()
	if err != nil {
		log.Error(err)
		_, err = os.Stdout.WriteString("unknown\n")
	} else {
		_, err = os.Stdout.WriteString(currentTag + "\n")
	}
	if err != nil {
		log.Error(err)
	}
}
