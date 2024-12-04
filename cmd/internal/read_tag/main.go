package main

import (
	"flag"
	"os"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
)

var nightly bool

func init() {
	flag.BoolVar(&nightly, "nightly", false, "Print nightly tag")
}

func main() {
	flag.Parse()
	var (
		tag string
		err error
	)
	if nightly {
		tag, err = build_shared.ReadTagNightly()
	} else {
		tag, err = build_shared.ReadTag()
	}
	if err != nil {
		log.Error(err)
		_, err = os.Stdout.WriteString("unknown\n")
	} else {
		_, err = os.Stdout.WriteString(tag + "\n")
	}
	if err != nil {
		log.Error(err)
	}
}
