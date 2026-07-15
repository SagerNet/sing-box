package main

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceDisplayName             = "sing-box Service"
	serviceDescriptionText         = "Privileged service for sing-box"
	defaultServiceWorkingDirectory = `C:\ProgramData\sing-box-daemon`
)

var commandServiceFlagAllowUnsafeInstallation bool

var commandServiceInstall = &cobra.Command{
	Use:   "install",
	Short: "Install or update the system service",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := serviceInstall()
		if err != nil {
			log.Fatal(E.Cause(err, "install service"))
		}
	},
}

var commandServiceUninstall = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the system service",
	Args:  cobra.NoArgs,
	Run: func(command *cobra.Command, args []string) {
		err := serviceUninstall()
		if err != nil {
			log.Fatal(E.Cause(err, "uninstall service"))
		}
	},
}

func addPlatformServiceCommands() {
	commandServiceInstall.Flags().BoolVar(
		&commandServiceFlagAllowUnsafeInstallation,
		"allow-unsafe-installation-directory-permissions",
		false,
		"skip installation path security validation and permission hardening",
	)
	commandService.AddCommand(commandServiceInstall)
	commandService.AddCommand(commandServiceUninstall)
}

func serviceInstall() error {
	executablePath, err := os.Executable()
	if err != nil {
		return E.Cause(err, "get executable path")
	}
	serviceWorkingDirectory, err := resolveWindowsServiceWorkingDirectory(commandServiceFlagWorkingDirectory)
	if err != nil {
		return E.Cause(err, "validate working directory")
	}
	executablePath, err = secureWindowsInstallation(executablePath, commandServiceFlagAllowUnsafeInstallation)
	if err != nil {
		return E.Cause(err, "secure installation")
	}
	err = ensureWindowsWorkingDirectory(serviceWorkingDirectory)
	if err != nil {
		return E.Cause(err, "secure working directory")
	}
	manager, err := mgr.Connect()
	if err != nil {
		return E.Cause(err, "connect to service manager")
	}
	defer manager.Disconnect()
	arguments := []string{"run", "--working-directory", serviceWorkingDirectory}
	config := mgr.Config{
		DisplayName:  serviceDisplayName,
		Description:  serviceDescriptionText,
		StartType:    mgr.StartAutomatic,
		Dependencies: []string{"Tcpip"},
		SidType:      windows.SERVICE_SID_TYPE_UNRESTRICTED,
	}
	created := false
	service, err := manager.OpenService(serviceName)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return E.Cause(err, "open service")
		}
		service, err = manager.CreateService(serviceName, executablePath, config, arguments...)
		if err != nil {
			return E.Cause(err, "create service")
		}
		created = true
	} else {
		err = updateServiceConfig(service, config, executablePath, arguments)
		if err != nil {
			service.Close()
			return err
		}
	}
	defer service.Close()
	rollback := func() {
		if created {
			_ = service.Delete()
		}
	}
	installedConfig, err := service.Config()
	if err != nil {
		rollback()
		return E.Cause(err, "query installed service config")
	}
	if installedConfig.SidType != windows.SERVICE_SID_TYPE_UNRESTRICTED {
		rollback()
		return E.New("unexpected installed service SID type: ", installedConfig.SidType)
	}
	err = service.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
	}, 86400)
	if err != nil {
		rollback()
		return E.Cause(err, "set recovery actions")
	}
	err = applyProtectedServiceSecurity(service)
	if err != nil {
		rollback()
		return E.Cause(err, "secure service")
	}
	err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		rollback()
		return E.Cause(err, "install event log source")
	}
	if !created {
		err = stopServiceAndWait(service)
		if err != nil {
			return E.Cause(err, "stop service")
		}
	}
	err = startServiceAndWait(service)
	if err != nil {
		return E.Cause(err, "start service")
	}
	return nil
}

func updateServiceConfig(service *mgr.Service, config mgr.Config, executablePath string, arguments []string) error {
	binaryPathName := windows.ComposeCommandLine(append([]string{executablePath}, arguments...))
	currentConfig, err := service.Config()
	if err != nil {
		return E.Cause(err, "query service config")
	}
	currentConfig.DisplayName = config.DisplayName
	currentConfig.Description = config.Description
	currentConfig.StartType = config.StartType
	currentConfig.Dependencies = config.Dependencies
	currentConfig.SidType = config.SidType
	currentConfig.BinaryPathName = binaryPathName
	err = service.UpdateConfig(currentConfig)
	if err != nil {
		return E.Cause(err, "update service config")
	}
	return nil
}

func serviceUninstall() error {
	manager, err := mgr.Connect()
	if err != nil {
		return E.Cause(err, "connect to service manager")
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			log.Info("service not installed")
			return nil
		}
		return E.Cause(err, "open service")
	}
	defer service.Close()
	err = stopServiceAndWait(service)
	if err != nil {
		log.Warn("stop service: ", err)
	}
	err = service.Delete()
	if err != nil {
		return E.Cause(err, "delete service")
	}
	err = eventlog.Remove(serviceName)
	if err != nil {
		log.Warn("remove event log source: ", err)
	}
	return nil
}

func serviceStart() error {
	manager, err := mgr.Connect()
	if err != nil {
		return E.Cause(err, "connect to service manager")
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if err != nil {
		return E.Cause(err, "open service")
	}
	defer service.Close()
	return startServiceAndWait(service)
}

func serviceStop() error {
	manager, err := mgr.Connect()
	if err != nil {
		return E.Cause(err, "connect to service manager")
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			log.Info("service not installed")
			return nil
		}
		return E.Cause(err, "open service")
	}
	defer service.Close()
	return stopServiceAndWait(service)
}

func startServiceAndWait(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return E.Cause(err, "query service status")
	}
	if status.State == svc.Running {
		return nil
	}
	if status.State == svc.Stopped {
		err = service.Start()
		if err != nil {
			return E.Cause(err, "start service")
		}
	}
	return waitServiceState(service, svc.Running)
}

func stopServiceAndWait(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return E.Cause(err, "query service status")
	}
	if status.State == svc.Stopped {
		return nil
	}
	if status.State != svc.StopPending {
		_, err = service.Control(svc.Stop)
		if err != nil {
			return E.Cause(err, "stop service")
		}
	}
	return waitServiceState(service, svc.Stopped)
}

func waitServiceState(service *mgr.Service, state svc.State) error {
	timeout := time.Now().Add(30 * time.Second)
	var currentStatus svc.Status
	for time.Now().Before(timeout) {
		status, err := service.Query()
		if err != nil {
			return E.Cause(err, "query service status")
		}
		currentStatus = status
		if status.State == state {
			return nil
		}
		if state == svc.Running && status.State == svc.Stopped {
			return E.New(
				"service stopped while starting, Windows exit code ", status.Win32ExitCode,
				", service exit code ", status.ServiceSpecificExitCode,
			)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return E.New(
		"timeout waiting for service state ", uint32(state),
		", current state ", uint32(currentStatus.State),
		", process ID ", currentStatus.ProcessId,
		", Windows exit code ", currentStatus.Win32ExitCode,
		", service exit code ", currentStatus.ServiceSpecificExitCode,
	)
}

func serviceStatus() (*serviceStatusResult, error) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, E.Cause(err, "connect to service manager")
	}
	defer windows.CloseServiceHandle(manager)
	namePointer, err := windows.UTF16PtrFromString(serviceName)
	if err != nil {
		return nil, err
	}
	service, err := windows.OpenService(manager, namePointer, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return &serviceStatusResult{exitCode: 3, description: "not installed"}, nil
		}
		return nil, E.Cause(err, "open service")
	}
	defer windows.CloseServiceHandle(service)
	var status windows.SERVICE_STATUS
	err = windows.QueryServiceStatus(service, &status)
	if err != nil {
		return nil, E.Cause(err, "query service status")
	}
	if status.CurrentState == windows.SERVICE_RUNNING {
		return &serviceStatusResult{exitCode: 0, description: "running"}, nil
	}
	return &serviceStatusResult{exitCode: 2, description: "stopped"}, nil
}
