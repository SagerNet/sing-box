package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "github.com/sagernet/gomobile/event/key"
	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/rw"
)

var (
	debugEnabled bool
	target       string
)

func init() {
	flag.BoolVar(&debugEnabled, "debug", false, "enable debug")
	flag.StringVar(&target, "target", "android", "target platform")
}

func main() {
	flag.Parse()

	build_shared.FindMobile()

	switch target {
	case "android":
		buildAndroid()
	case "ios":
		buildiOS()
	}
}

var (
	sharedFlags []string
	debugFlags  []string
	sharedTags  []string
	debugTags   []string
)

func init() {
	sharedFlags = append(sharedFlags, "-trimpath")
	sharedFlags = append(sharedFlags, "-ldflags")
	currentTag, err := build_shared.ReadTag()
	if err != nil {
		currentTag = "unknown"
	}
	sharedFlags = append(sharedFlags, "-X github.com/sagernet/sing-box/constant.Version="+currentTag+" -s -w -buildid=")
	debugFlags = append(debugFlags, "-X github.com/sagernet/sing-box/constant.Version="+currentTag)

	sharedTags = append(sharedTags, "with_gvisor", "with_quic", "with_wireguard", "with_utls", "with_clash_api")
	sharedTags = append(sharedTags, "test_sing_shadowsocks2")
	debugTags = append(debugTags, "debug")
}

func buildAndroid() {
	build_shared.FindSDK()

	args := []string{
		"bind",
		"-v",
		"-androidapi", "21",
		"-javapkg=io.nekohasekai",
		"-libname=box",
	}
	if !debugEnabled {
		args = append(args, sharedFlags...)
	} else {
		args = append(args, debugFlags...)
	}

	args = append(args, "-tags")
	if !debugEnabled {
		args = append(args, strings.Join(sharedTags, ","))
	} else {
		args = append(args, strings.Join(append(sharedTags, debugTags...), ","))
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

func buildiOS() {
	args := []string{
		"bind",
		"-v",
		"-target", "ios,iossimulator,macos",
		"-libname=box",
	}
	if !debugEnabled {
		args = append(args, sharedFlags...)
	} else {
		args = append(args, debugFlags...)
	}

	tags := append(sharedTags, "with_low_memory", "with_conntrack")
	args = append(args, "-tags")
	if !debugEnabled {
		args = append(args, strings.Join(tags, ","))
	} else {
		args = append(args, strings.Join(append(tags, debugTags...), ","))
	}
	args = append(args, "./experimental/libbox")

	command := exec.Command(build_shared.GoBinPath+"/gomobile", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	copyPath := filepath.Join("..", "sing-box-for-ios")
	if rw.FileExists(copyPath) {
		targetDir := filepath.Join(copyPath, "Libbox.xcframework")
		targetDir, _ = filepath.Abs(targetDir)
		os.RemoveAll(targetDir)
		os.Rename("Libbox.xcframework", targetDir)
		log.Info("copied to ", targetDir)
	}
}
