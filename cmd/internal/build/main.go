package main

import (
	"os"
	"os/exec"

	"github.com/sagernet/sing-box/log"
)

func main() {
	findSDK()

	command := exec.Command(os.Args[1], os.Args[2:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}
}
