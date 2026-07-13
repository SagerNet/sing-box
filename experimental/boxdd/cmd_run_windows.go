package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

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
	if !strings.EqualFold(filepath.Clean(workingDirectory), filepath.Clean(defaultServiceWorkingDirectory)) {
		return E.New("the Windows service working directory must be ", defaultServiceWorkingDirectory)
	}
	return ensureWindowsWorkingDirectory(workingDirectory)
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
		serviceLogError(E.New("unexpected service command: ", request.Cmd))
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
