package main

import (
	"os"
	"path/filepath"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/tailscale/atomicfile"
)

const (
	serviceConfigFileName = "config.json"
	startOptionsFileName  = "start_options.json"
)

type startOptions struct {
	WasRunning        bool   `json:"was_running"`
	OwnerUserID       string `json:"owner_user_id"`
	OOMKillerEnabled  bool   `json:"oom_killer_enabled"`
	OOMKillerDisabled bool   `json:"oom_killer_disabled"`
	OOMMemoryLimit    int64  `json:"oom_memory_limit"`
}

func loadServiceConfig() (string, error) {
	content, err := os.ReadFile(filepath.Join(workingDirectory, serviceConfigFileName))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func loadStartOptions() (startOptions, error) {
	content, err := os.ReadFile(filepath.Join(workingDirectory, startOptionsFileName))
	if err != nil {
		return startOptions{}, err
	}
	options, err := json.UnmarshalExtended[startOptions](content)
	if err != nil {
		return startOptions{}, err
	}
	return options, nil
}

func saveStartOptions(options startOptions) error {
	content, err := json.Marshal(options)
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(workingDirectory, startOptionsFileName), content, 0o600)
}
