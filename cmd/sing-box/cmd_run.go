package main

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	runtimeDebug "runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/spf13/cobra"
)

var commandRun = &cobra.Command{
	Use:   "run",
	Short: "Run service",
	Run: func(cmd *cobra.Command, args []string) {
		err := run()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	mainCommand.AddCommand(commandRun)
}

type OptionsEntry struct {
	content []byte
	path    string
	options option.Options
}

const (
	configAPIBaseURLEnv = "SINGBOX_CONFIG_API_BASE_URL"
	configAPINodeIDEnv  = "SINGBOX_CONFIG_NODE_ID"
	configAPIRetryCount = 5
)

type remoteConfigResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    stdjson.RawMessage `json:"data"`
}

func mustRemoteConfigRequest() (string, int) {
	baseURL, exists := os.LookupEnv(configAPIBaseURLEnv)
	if !exists || strings.TrimSpace(baseURL) == "" {
		panic(fmt.Sprintf("missing environment variable %s", configAPIBaseURLEnv))
	}
	nodeIDText, exists := os.LookupEnv(configAPINodeIDEnv)
	if !exists || strings.TrimSpace(nodeIDText) == "" {
		panic(fmt.Sprintf("missing environment variable %s", configAPINodeIDEnv))
	}
	nodeID, err := strconv.Atoi(nodeIDText)
	if err != nil {
		panic(fmt.Sprintf("invalid environment variable %s: %v", configAPINodeIDEnv, err))
	}
	return strings.TrimRight(baseURL, "/"), nodeID
}

func fetchRemoteConfig() (option.Options, error) {
	baseURL, nodeID := mustRemoteConfigRequest()
	requestBody, err := stdjson.Marshal(map[string]int{
		"id": nodeID,
	})
	if err != nil {
		return option.Options{}, E.Cause(err, "marshal remote config request")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= configAPIRetryCount; attempt++ {
		options, fetchErr := fetchRemoteConfigOnce(client, baseURL, requestBody)
		if fetchErr == nil {
			return options, nil
		}
		lastErr = fetchErr
		log.Error(E.Cause(fetchErr, "fetch remote config attempt ", attempt, "/", configAPIRetryCount))
		if attempt < configAPIRetryCount {
			time.Sleep(time.Second)
		}
	}
	return option.Options{}, E.Cause(lastErr, "fetch remote config failed after ", configAPIRetryCount, " attempts")
}

func fetchRemoteConfigOnce(client *http.Client, baseURL string, requestBody []byte) (option.Options, error) {
	request, err := http.NewRequestWithContext(globalCtx, http.MethodPost, baseURL+"/api/v1/nodes/config", bytes.NewReader(requestBody))
	if err != nil {
		return option.Options{}, E.Cause(err, "create remote config request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("from", "node")
	response, err := client.Do(request)
	if err != nil {
		return option.Options{}, E.Cause(err, "request remote config")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return option.Options{}, E.Cause(err, "read remote config response")
	}
	if response.StatusCode != http.StatusOK {
		return option.Options{}, E.New("remote config api status ", response.StatusCode, ": ", strings.TrimSpace(string(responseBody)))
	}
	var configResponse remoteConfigResponse
	err = stdjson.Unmarshal(responseBody, &configResponse)
	if err != nil {
		return option.Options{}, E.Cause(err, "decode remote config response")
	}
	if configResponse.Code != 0 {
		return option.Options{}, E.New("remote config api error: code=", configResponse.Code, ", message=", configResponse.Message)
	}
	if len(configResponse.Data) == 0 || string(configResponse.Data) == "null" {
		return option.Options{}, E.New("remote config api returned empty data")
	}
	options, err := json.UnmarshalExtendedContext[option.Options](globalCtx, configResponse.Data)
	if err != nil {
		return option.Options{}, E.Cause(err, "decode remote config data")
	}
	return options, nil
}

func readConfigAt(path string) (*OptionsEntry, error) {
	var (
		configContent []byte
		err           error
	)
	if path == "stdin" {
		configContent, err = io.ReadAll(os.Stdin)
	} else {
		configContent, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, E.Cause(err, "read config at ", path)
	}
	options, err := json.UnmarshalExtendedContext[option.Options](globalCtx, configContent)
	if err != nil {
		return nil, E.Cause(err, "decode config at ", path)
	}
	return &OptionsEntry{
		content: configContent,
		path:    path,
		options: options,
	}, nil
}

func readConfig() ([]*OptionsEntry, error) {
	ensureDefaultConfigPath()
	var optionsList []*OptionsEntry
	for _, path := range configPaths {
		optionsEntry, err := readConfigAt(path)
		if err != nil {
			return nil, err
		}
		optionsList = append(optionsList, optionsEntry)
	}
	for _, directory := range configDirectories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, E.Cause(err, "read config directory at ", directory)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
				continue
			}
			optionsEntry, err := readConfigAt(filepath.Join(directory, entry.Name()))
			if err != nil {
				return nil, err
			}
			optionsList = append(optionsList, optionsEntry)
		}
	}
	sort.Slice(optionsList, func(i, j int) bool {
		return optionsList[i].path < optionsList[j].path
	})
	return optionsList, nil
}

func readConfigAndMerge() (option.Options, error) {
	optionsList, err := readConfig()
	if err != nil {
		return option.Options{}, err
	}
	if len(optionsList) == 1 {
		return optionsList[0].options, nil
	}
	var mergedMessage json.RawMessage
	for _, options := range optionsList {
		mergedMessage, err = badjson.MergeJSON(globalCtx, options.options.RawMessage, mergedMessage, false)
		if err != nil {
			return option.Options{}, E.Cause(err, "merge config at ", options.path)
		}
	}
	var mergedOptions option.Options
	err = mergedOptions.UnmarshalJSONContext(globalCtx, mergedMessage)
	if err != nil {
		return option.Options{}, E.Cause(err, "unmarshal merged config")
	}
	return mergedOptions, nil
}

func create() (*box.Box, context.CancelFunc, error) {
	options, err := fetchRemoteConfig()
	if err != nil {
		return nil, nil, err
	}
	if disableColor {
		if options.Log == nil {
			options.Log = &option.LogOptions{}
		}
		options.Log.DisableColor = true
	}
	ctx, cancel := context.WithCancel(globalCtx)
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		cancel()
		return nil, nil, E.Cause(err, "create service")
	}

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		signal.Stop(osSignals)
		close(osSignals)
	}()
	startCtx, finishStart := context.WithCancel(context.Background())
	go func() {
		_, loaded := <-osSignals
		if loaded {
			cancel()
			closeMonitor(startCtx)
		}
	}()
	err = instance.Start()
	finishStart()
	if err != nil {
		cancel()
		return nil, nil, E.Cause(err, "start service")
	}
	return instance, cancel, nil
}

func run() error {
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(osSignals)
	for {
		instance, cancel, err := create()
		if err != nil {
			return err
		}
		runtimeDebug.FreeOSMemory()
		for {
			osSignal := <-osSignals
			if osSignal == syscall.SIGHUP {
				err = check()
				if err != nil {
					log.Error(E.Cause(err, "reload service"))
					continue
				}
			}
			cancel()
			closeCtx, closed := context.WithCancel(context.Background())
			go closeMonitor(closeCtx)
			err = instance.Close()
			closed()
			if osSignal != syscall.SIGHUP {
				if err != nil {
					log.Error(E.Cause(err, "sing-box did not closed properly"))
				}
				return nil
			}
			break
		}
	}
}

func closeMonitor(ctx context.Context) {
	time.Sleep(C.FatalStopTimeout)
	select {
	case <-ctx.Done():
		return
	default:
	}
	log.Fatal("sing-box did not close!")
}
