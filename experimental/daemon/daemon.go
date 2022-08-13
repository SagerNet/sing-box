package daemon

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"

	"github.com/kardianos/service"
	C "github.com/sagernet/sing-box/constant"
)

const (
	DefaultDaemonName = "sing-box-daemon"
	DefaultDaemonPort = 9091
)

var defaultDaemonOptions = Options{
	Listen:           "127.0.0.1",
	ListenPort:       DefaultDaemonPort,
	WorkingDirectory: workingDirectory(),
}

func workingDirectory() string {
	switch runtime.GOOS {
	case "linux":
		return filepath.Join("/usr/local/lib", DefaultDaemonName)
	default:
		configDir, err := os.UserConfigDir()
		if err == nil {
			return filepath.Join(configDir, DefaultDaemonName)
		} else {
			return DefaultDaemonName
		}
	}
}

const systemdScript = `[Unit]
Description=sing-box service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
User=root
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
WorkingDirectory={{.WorkingDirectory|cmdEscape}}
Restart=on-failure
RestartSec=10s
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target`

type Daemon struct {
	service          service.Service
	workingDirectory string
	executable       string
}

func New() (*Daemon, error) {
	daemonInterface := NewInterface(defaultDaemonOptions)
	executable := filepath.Join(defaultDaemonOptions.WorkingDirectory, "sing-box")
	if C.IsWindows {
		executable += ".exe"
	}
	daemonService, err := service.New(daemonInterface, &service.Config{
		Name:        DefaultDaemonName,
		Description: "The universal proxy platform.",
		Arguments:   []string{"daemon", "run"},
		Executable:  executable,
		Option: service.KeyValue{
			"SystemdScript": systemdScript,
		},
	})
	if err != nil {
		return nil, E.New(strings.ToLower(err.Error()))
	}
	return &Daemon{
		service:          daemonService,
		workingDirectory: defaultDaemonOptions.WorkingDirectory,
		executable:       executable,
	}, nil
}

func (d *Daemon) Install() error {
	_, err := d.service.Status()
	if err != service.ErrNotInstalled {
		d.service.Stop()
		err = d.service.Uninstall()
		if err != nil {
			return err
		}
	}
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	if !rw.FileExists(d.workingDirectory) {
		err = os.MkdirAll(d.workingDirectory, 0o755)
		if err != nil {
			return err
		}
	}
	outputFile, err := os.OpenFile(d.executable, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	inputFile, err := os.Open(executablePath)
	if err != nil {
		outputFile.Close()
		return err
	}
	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	outputFile.Close()
	if err != nil {
		return err
	}
	err = d.service.Install()
	if err != nil {
		return err
	}
	return d.service.Start()
}

func (d *Daemon) Uninstall() error {
	_, err := d.service.Status()
	if err != service.ErrNotInstalled {
		d.service.Stop()
		err = d.service.Uninstall()
		if err != nil {
			return err
		}
	}
	return os.RemoveAll(d.workingDirectory)
}

func (d *Daemon) Run() error {
	d.chdir()
	return d.service.Run()
}

func (d *Daemon) chdir() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return os.Chdir(filepath.Dir(executable))
}

func (d *Daemon) Start() error {
	return d.service.Start()
}

func (d *Daemon) Stop() error {
	return d.service.Stop()
}

func (d *Daemon) Restart() error {
	return d.service.Restart()
}
