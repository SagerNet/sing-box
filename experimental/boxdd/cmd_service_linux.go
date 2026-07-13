package main

import (
	"os/exec"
	"strings"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

const (
	defaultServiceWorkingDirectory = "/var/lib/sing-box-daemon"
	serviceUnitName                = serviceName + ".service"
)

var commandServiceRestart = &cobra.Command{
	Use:   "restart",
	Short: "Restart the system service",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := serviceRestart()
		if err != nil {
			log.Fatal(E.Cause(err, "restart service"))
		}
	},
}

func addPlatformServiceCommands() {
	commandService.AddCommand(commandServiceRestart)
}

func runSystemctl(arguments ...string) error {
	output, err := exec.Command("systemctl", arguments...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return E.Cause(err, "systemctl ", strings.Join(arguments, " "))
		}
		return E.New("systemctl ", strings.Join(arguments, " "), ": ", message)
	}
	return nil
}

func serviceStart() error {
	return runSystemctl("start", serviceUnitName)
}

func serviceStop() error {
	installed, err := serviceInstalled()
	if err != nil {
		return err
	}
	if !installed {
		log.Info("service not installed")
		return nil
	}
	return runSystemctl("stop", serviceUnitName)
}

func serviceRestart() error {
	return runSystemctl("restart", serviceUnitName)
}

func serviceInstalled() (bool, error) {
	loadState, err := systemctlProperty("LoadState")
	if err != nil {
		return false, err
	}
	return loadState != "" && loadState != "not-found", nil
}

func systemctlProperty(property string) (string, error) {
	output, err := exec.Command("systemctl", "show", "--property="+property, "--value", serviceUnitName).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", E.Cause(err, "query system service")
		}
		return "", E.New("query system service: ", message)
	}
	return strings.TrimSpace(string(output)), nil
}

func serviceStatus() (*serviceStatusResult, error) {
	installed, err := serviceInstalled()
	if err != nil {
		return nil, err
	}
	if !installed {
		return &serviceStatusResult{exitCode: 3, description: "not installed"}, nil
	}
	activeState, err := systemctlProperty("ActiveState")
	if err != nil {
		return nil, err
	}
	if activeState == "active" {
		return &serviceStatusResult{exitCode: 0, description: "running"}, nil
	}
	return &serviceStatusResult{exitCode: 2, description: "stopped"}, nil
}
