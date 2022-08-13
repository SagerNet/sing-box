//go:build with_daemon

package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/experimental/daemon"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
)

var commandDaemon = &cobra.Command{
	Use: "daemon",
}

func init() {
	commandDaemon.AddCommand(commandDaemonInstall)
	commandDaemon.AddCommand(commandDaemonUninstall)
	commandDaemon.AddCommand(commandDaemonStart)
	commandDaemon.AddCommand(commandDaemonStop)
	commandDaemon.AddCommand(commandDaemonRestart)
	commandDaemon.AddCommand(commandDaemonRun)
	mainCommand.AddCommand(commandDaemon)
	mainCommand.AddCommand(commandStart)
	mainCommand.AddCommand(commandStop)
	mainCommand.AddCommand(commandStatus)
}

var commandDaemonInstall = &cobra.Command{
	Use:   "install",
	Short: "Install daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := installDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandDaemonUninstall = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := uninstallDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandDaemonStart = &cobra.Command{
	Use:   "start",
	Short: "Start daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := startDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandDaemonStop = &cobra.Command{
	Use:   "stop",
	Short: "Stop daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := stopDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandDaemonRestart = &cobra.Command{
	Use:   "restart",
	Short: "Restart daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := restartDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandDaemonRun = &cobra.Command{
	Use:   "run",
	Short: "Run daemon",
	Run: func(cmd *cobra.Command, args []string) {
		err := runDaemon()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func installDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Install()
}

func uninstallDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Uninstall()
}

func startDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Start()
}

func stopDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Stop()
}

func restartDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Restart()
}

func runDaemon() error {
	instance, err := daemon.New()
	if err != nil {
		return err
	}
	return instance.Run()
}

var commandStart = &cobra.Command{
	Use:   "start",
	Short: "Start service",
	Run: func(cmd *cobra.Command, args []string) {
		err := startService()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandStop = &cobra.Command{
	Use:   "stop",
	Short: "Stop service",
	Run: func(cmd *cobra.Command, args []string) {
		err := stopService()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

var commandStatus = &cobra.Command{
	Use:   "status",
	Short: "Check service",
	Run: func(cmd *cobra.Command, args []string) {
		err := checkService()
		if err != nil {
			log.Fatal(err)
		}
	},
	Args: cobra.NoArgs,
}

func doRequest(method string, path string, params url.Values, body io.ReadCloser) ([]byte, error) {
	requestURL := url.URL{
		Scheme: "http",
		Path:   path,
		Host:   net.JoinHostPort("127.0.0.1", F.ToString(daemon.DefaultDaemonPort)),
	}
	if params != nil {
		requestURL.RawQuery = params.Encode()
	}
	request, err := http.NewRequest(method, requestURL.String(), body)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var content []byte
	if response.StatusCode != http.StatusNoContent {
		content, err = io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
		return nil, E.New(string(content))
	}
	return content, nil
}

func ping() error {
	response, err := doRequest("GET", "/ping", nil, nil)
	if err != nil || string(response) != "pong" {
		return E.New("daemon not running")
	}
	return nil
}

func startService() error {
	if err := ping(); err != nil {
		return err
	}
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return E.Cause(err, "read config")
	}
	return common.Error(doRequest("POST", "/run", nil, io.NopCloser(bytes.NewReader(configContent))))
}

func stopService() error {
	if err := ping(); err != nil {
		return err
	}
	return common.Error(doRequest("GET", "/stop", nil, nil))
}

func checkService() error {
	if err := ping(); err != nil {
		return err
	}
	response, err := doRequest("GET", "/status", nil, nil)
	if err != nil {
		return err
	}
	var statusResponse daemon.StatusResponse
	err = json.Unmarshal(response, &statusResponse)
	if err != nil {
		return err
	}
	if statusResponse.Running {
		log.Info("service running")
	} else {
		log.Info("service stopped")
	}
	return nil
}
