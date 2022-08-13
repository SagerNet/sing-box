package main

import (
	"os"
	"runtime"

	C "github.com/sagernet/sing-box/constant"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandVersion = &cobra.Command{
	Use:   "version",
	Short: "Print current version of sing-box",
	Run:   printVersion,
	Args:  cobra.NoArgs,
}

var nameOnly bool

func init() {
	commandVersion.Flags().BoolVarP(&nameOnly, "name", "n", false, "print version name only")
	mainCommand.AddCommand(commandVersion)
}

func printVersion(cmd *cobra.Command, args []string) {
	var version string
	if !nameOnly {
		version = "sing-box "
	}
	version += F.ToString(C.Version)
	if C.Commit != "" {
		version += "." + C.Commit
	}
	if !nameOnly {
		version += " ("
		version += runtime.Version()
		version += ", "
		version += runtime.GOOS
		version += ", "
		version += runtime.GOARCH
		version += ", "
		version += "CGO "
		if C.CGO_ENABLED {
			version += "enabled"
		} else {
			version += "disabled"
		}
		version += ")"
	}
	version += "\n"
	os.Stdout.WriteString(version)
}
