package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	debugEnabled bool
	outputPath   string
	target       string
)

func init() {
	flag.BoolVar(&debugEnabled, "debug", false, "enable debug")
	flag.StringVar(&outputPath, "output", "", "output path")
	flag.StringVar(&target, "target", runtime.GOOS+"/"+runtime.GOARCH, "target platform")
}

func main() {
	flag.Parse()
	err := build()
	if err != nil {
		log.Fatal(err)
	}
}

func build() error {
	targetParts := strings.Split(target, "/")
	if len(targetParts) != 2 || targetParts[0] == "" || targetParts[1] == "" {
		return E.New("invalid target: ", target)
	}
	operatingSystem := targetParts[0]
	architecture := targetParts[1]
	if outputPath == "" {
		outputPath = "sing-box-daemon"
		if operatingSystem == "windows" {
			outputPath += ".exe"
		}
	}
	absoluteOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return E.Cause(err, "resolve output path")
	}
	err = os.MkdirAll(filepath.Dir(absoluteOutputPath), 0o755)
	if err != nil {
		return E.Cause(err, "create output directory")
	}
	version, err := build_shared.ReadTag()
	if err != nil {
		return E.Cause(err, "read version")
	}
	tags, err := buildTags(operatingSystem, architecture)
	if err != nil {
		return err
	}
	arguments := []string{
		"build",
		"-v",
		"-trimpath",
		"-buildvcs=false",
		"-tags", strings.Join(tags, ","),
		"-ldflags", build_shared.LinkerFlags(version, debugEnabled),
		"-o", absoluteOutputPath,
	}
	if operatingSystem == "windows" && architecture == "386" {
		arguments = append(arguments, "-gcflags=net=-l")
	}
	arguments = append(arguments, "./experimental/boxdd")
	command := exec.Command("go", arguments...)
	command.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+operatingSystem,
		"GOARCH="+architecture,
		"GOTOOLCHAIN=local",
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		return E.Cause(err, "build sing-box daemon")
	}
	return nil
}

func buildTags(operatingSystem string, architecture string) ([]string, error) {
	tagsFile := "release/DEFAULT_BUILD_TAGS"
	if operatingSystem == "windows" {
		if architecture == "386" {
			tagsFile = "release/DEFAULT_BUILD_TAGS_OTHERS"
		} else {
			tagsFile = "release/DEFAULT_BUILD_TAGS_WINDOWS"
		}
	}
	content, err := os.ReadFile(tagsFile)
	if err != nil {
		return nil, E.Cause(err, "read build tags")
	}
	tags := strings.Split(strings.TrimSpace(string(content)), ",")
	if operatingSystem != "windows" {
		tags = append(tags, "with_purego")
	}
	if debugEnabled {
		tags = append(tags, "debug")
	}
	return tags, nil
}
