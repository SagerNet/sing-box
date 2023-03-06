package main

import (
	"os"
	"os/exec"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
)

func main() {
	build_shared.FindSDK()

	currentTag, err := common.Exec("git", "describe", "--tags", "--abbrev=0").Read()
	if err != nil {
		log.Fatal(err)
	}

	if "v"+C.Version != currentTag {
		log.Fatal("version mismatch, update constant.Version to ", currentTag[1:])
	}

	command := exec.Command(os.Args[1], os.Args[2:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		log.Fatal(err)
	}
}
