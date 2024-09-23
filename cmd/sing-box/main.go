//go:build !generate

package main

import "github.com/sagernet/sing-box/log"

func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}
