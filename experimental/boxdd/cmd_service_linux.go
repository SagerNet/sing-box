package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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

var commandServiceSetInsecureMode = &cobra.Command{
	Use:   "set-insecure-mode <enabled>",
	Short: "Set whether configurations may use privileges unrelated to networking",
	Args:  cobra.ExactArgs(1),
	Run: func(command *cobra.Command, args []string) {
		err := serviceSetInsecureMode(args[0])
		if err != nil {
			log.Fatal(E.Cause(err, "set insecure mode"))
		}
	},
}

func addPlatformServiceCommands() {
	commandService.AddCommand(commandServiceRestart)
	commandService.AddCommand(commandServiceSetInsecureMode)
}

func serviceSetInsecureMode(value string) error {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return E.Cause(err, "parse value")
	}
	if os.Geteuid() != 0 {
		return E.New("setting insecure mode requires an elevated process")
	}
	directory, err := filepath.Abs(commandServiceFlagWorkingDirectory)
	if err != nil {
		return E.Cause(err, "resolve working directory")
	}
	err = validateProtectedLinuxDirectory(directory)
	if err != nil {
		return E.Cause(err, "validate working directory")
	}
	return saveSecuritySettings(directory, securitySettings{InsecureModeEnabled: enabled})
}

func validateProtectedLinuxDirectory(directory string) error {
	currentPath := directory
	for {
		info, err := os.Lstat(currentPath)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return E.New("protected path is not a directory: ", currentPath)
		}
		fileStatus, loaded := info.Sys().(*syscall.Stat_t)
		if !loaded || fileStatus.Uid != 0 {
			return E.New("protected path is not owned by root: ", currentPath)
		}
		if info.Mode().Perm()&0o022 != 0 {
			return E.New("protected path is writable by non-root users: ", currentPath)
		}
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			return nil
		}
		currentPath = parentPath
	}
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
