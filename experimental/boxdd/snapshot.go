package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/tailscale/atomicfile"
)

const (
	serviceConfigFileName = "config.json"
	startOptionsFileName  = "start_options.json"
	ownerFileName         = "owner.json"
	usersDirectoryName    = "users"
)

type startOptions struct {
	WasRunning        bool  `json:"was_running"`
	OOMKillerEnabled  bool  `json:"oom_killer_enabled"`
	OOMKillerDisabled bool  `json:"oom_killer_disabled"`
	OOMMemoryLimit    int64 `json:"oom_memory_limit"`
}

type ownerState struct {
	UserID string `json:"user_id"`
}

func userWorkingDirectory(userID string) string {
	digest := sha256.Sum256([]byte(userID))
	return filepath.Join(workingDirectory, usersDirectoryName, hex.EncodeToString(digest[:]))
}

func loadOwner() (string, error) {
	content, err := os.ReadFile(filepath.Join(workingDirectory, ownerFileName))
	if err != nil {
		return "", err
	}
	state, err := json.UnmarshalExtended[ownerState](content)
	if err != nil {
		return "", err
	}
	return state.UserID, nil
}

func saveOwner(userID string) error {
	content, err := json.Marshal(ownerState{UserID: userID})
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(workingDirectory, ownerFileName), content, 0o600)
}

func loadServiceConfig(userID string) (string, error) {
	content, err := os.ReadFile(filepath.Join(userWorkingDirectory(userID), serviceConfigFileName))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func loadStartOptions(userID string) (startOptions, error) {
	content, err := os.ReadFile(filepath.Join(userWorkingDirectory(userID), startOptionsFileName))
	if err != nil {
		return startOptions{}, err
	}
	options, err := json.UnmarshalExtended[startOptions](content)
	if err != nil {
		return startOptions{}, err
	}
	return options, nil
}

func saveStartOptions(userID string, options startOptions) error {
	content, err := json.Marshal(options)
	if err != nil {
		return err
	}
	directory := userWorkingDirectory(userID)
	err = os.MkdirAll(directory, 0o700)
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(filepath.Join(directory, startOptionsFileName), content, 0o600)
}
