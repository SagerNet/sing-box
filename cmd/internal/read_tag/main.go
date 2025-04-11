package main

import (
	"flag"
	"os"

	"github.com/sagernet/sing-box/cmd/internal/build_shared"
	"github.com/sagernet/sing-box/common/badversion"
	"github.com/sagernet/sing-box/log"
)

var (
	flagRunInCI    bool
	flagRunNightly bool
)

func init() {
	flag.BoolVar(&flagRunInCI, "ci", false, "Run in CI")
	flag.BoolVar(&flagRunNightly, "nightly", false, "Run nightly")
}

func main() {
	flag.Parse()
	var (
		versionStr string
		err        error
	)
	if flagRunNightly {
		var version badversion.Version
		version, err = build_shared.ReadTagVersion()
		if err == nil {
			versionStr = version.String()
		}
	} else {
		versionStr, err = build_shared.ReadTag()
	}
	if flagRunInCI {
		if err != nil {
			log.Fatal(err)
		}
		err = setGitHubEnv("version", versionStr)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if err != nil {
			log.Error(err)
			os.Stdout.WriteString("unknown\n")
		} else {
			os.Stdout.WriteString(versionStr + "\n")
		}
	}
}

func setGitHubEnv(name string, value string) error {
	outputFile, err := os.OpenFile(os.Getenv("GITHUB_ENV"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, err = outputFile.WriteString(name + "=" + value + "\n")
	if err != nil {
		outputFile.Close()
		return err
	}
	err = outputFile.Close()
	if err != nil {
		return err
	}
	os.Stderr.WriteString(name + "=" + value + "\n")
	return nil
}
