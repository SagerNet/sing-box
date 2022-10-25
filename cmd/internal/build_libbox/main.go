package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/sagernet/gomobile/asset"
	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/rw"
)

var debugEnabled bool

func init() {
	flag.BoolVar(&debugEnabled, "debug", false, "enable debug")
}

func main() {
	build_shared.FindSDK()
	build_shared.FindMobile()

	args := []string{
		"bind",
		"-v",
		"-androidapi", "21",
		"-javapkg=io.nekohasekai",
		"-libname=box",
	}
	if !debugEnabled {
		args = append(args,
			"-trimpath", "-ldflags=-s -w -buildid=",
			"-tags", "with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,debug",
		)
	} else {
		args = append(args, "-tags", "with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api")
	}

	args = append(args, "./experimental/libbox")

	command := exec.Command(build_shared.GoBinPath+"/gomobile", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	const name = "libbox.aar"
	copyPath := filepath.Join("..", "sing-box-for-android", "app", "libs")
	if rw.FileExists(copyPath) {
		copyPath, _ = filepath.Abs(copyPath)
		err = rw.CopyFile(name, filepath.Join(copyPath, name))
		if err != nil {
			log.Fatal(err)
		}
		log.Info("copied to ", copyPath)
	}
}
