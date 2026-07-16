package main

import (
	"os"
	"runtime"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

var commandRunFlagAllowUnsafeInstallation bool

func init() {
	commandRun.Flags().BoolVar(
		&commandRunFlagAllowUnsafeInstallation,
		"allow-unsafe-installation-directory-permissions",
		false,
		"allow unsafe daemon working directory ancestor permissions",
	)
}

func runService() (bool, error) {
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		return true, E.Cause(err, "check windows service")
	}
	if !isWindowsService {
		return false, nil
	}
	return true, svc.Run(serviceName, &windowsService{})
}

func preparePlatformWorkingDirectory() error {
	if listenAddress != "" {
		return os.MkdirAll(workingDirectory, 0o700)
	}
	serviceWorkingDirectory, err := resolveWindowsServiceWorkingDirectory(
		workingDirectory,
		commandRunFlagAllowUnsafeInstallation,
	)
	if err != nil {
		return err
	}
	workingDirectory = serviceWorkingDirectory
	return ensureWindowsWorkingDirectory(serviceWorkingDirectory)
}

type windowsService struct{}

func (s *windowsService) Execute(arguments []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (serviceSpecific bool, exitCode uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	if listenAddress != "" {
		exitCode = 1
		serviceLogError(E.New("--listen is not allowed in service mode"))
		return
	}
	err := allowAuthenticatedUsersToQueryCurrentProcess()
	if err != nil {
		exitCode = 1
		serviceLogError(E.Cause(err, "secure daemon process"))
		return
	}
	err = prepareWorkingDirectory()
	if err != nil {
		exitCode = 1
		serviceLogError(err)
		return
	}
	d, err := newDaemon()
	if err != nil {
		exitCode = 1
		serviceLogError(err)
		return
	}
	err = d.Start()
	if err != nil {
		exitCode = 1
		serviceLogError(err)
		return
	}
	statuses <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange}
	runtime.GC()
	for request := range requests {
		if request.Cmd == svc.Interrogate {
			statuses <- request.CurrentStatus
			continue
		}
		if request.Cmd == svc.SessionChange {
			err = d.handlePlatformSessionChange(request.EventType, uint32(request.EventData))
			if err != nil {
				serviceLogError(E.Cause(err, "handle session change"))
			}
			continue
		}
		if request.Cmd == svc.Stop || request.Cmd == svc.Shutdown {
			break
		}
		serviceLogError(E.New("unexpected service command: ", uint32(request.Cmd)))
	}
	statuses <- svc.Status{State: svc.StopPending}
	watchdog := time.AfterFunc(3*time.Second, func() {
		serviceLogError(E.New("daemon did not close"))
		os.Exit(1)
	})
	d.Close()
	watchdog.Stop()
	statuses <- svc.Status{State: svc.Stopped}
	return
}

func serviceLogError(err error) {
	eventLog, openError := eventlog.Open(serviceName)
	if openError != nil {
		return
	}
	eventLog.Error(1, F.ToString(err))
	eventLog.Close()
}
