package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/tailscale/atomicfile"

	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	dataProtectionDirectoryName = "data-protection"
	dataProtectionKeyLength     = 32
)

type dataProtectionState struct {
	Disabled bool   `json:"disabled,omitempty"`
	Key      string `json:"key,omitempty"`
}

func (s *desktopService) GetDataProtection(ctx context.Context, empty *emptypb.Empty) (*DataProtectionInfo, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.dataProtectionAccess.Lock()
	defer s.daemon.dataProtectionAccess.Unlock()
	return loadDataProtection(identity.UserID)
}

func (s *desktopService) SetDataProtection(ctx context.Context, request *SetDataProtectionRequest) (*DataProtectionInfo, error) {
	identity, err := peerIdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.daemon.dataProtectionAccess.Lock()
	defer s.daemon.dataProtectionAccess.Unlock()
	if !request.Enabled {
		err = saveDataProtectionState(identity.UserID, dataProtectionState{Disabled: true})
		if err != nil {
			return nil, err
		}
		return &DataProtectionInfo{}, nil
	}
	protection, err := loadDataProtection(identity.UserID)
	if err != nil {
		return nil, err
	}
	if protection.Enabled {
		return protection, nil
	}
	return createDataProtectionKey(identity.UserID)
}

func loadDataProtection(userID string) (*DataProtectionInfo, error) {
	state, err := loadDataProtectionState(userID)
	if os.IsNotExist(err) {
		return createDataProtectionKey(userID)
	}
	if err != nil {
		return nil, err
	}
	if state.Disabled {
		return &DataProtectionInfo{}, nil
	}
	key, err := hex.DecodeString(state.Key)
	if err != nil || len(key) != dataProtectionKeyLength {
		return createDataProtectionKey(userID)
	}
	return &DataProtectionInfo{Enabled: true, Key: key}, nil
}

func createDataProtectionKey(userID string) (*DataProtectionInfo, error) {
	key := make([]byte, dataProtectionKeyLength)
	_, err := rand.Read(key)
	if err != nil {
		return nil, E.Cause(err, "generate data protection key")
	}
	err = saveDataProtectionState(userID, dataProtectionState{Key: hex.EncodeToString(key)})
	if err != nil {
		return nil, err
	}
	return &DataProtectionInfo{Enabled: true, Key: key}, nil
}

func dataProtectionStatePath(userID string) string {
	return filepath.Join(workingDirectory, dataProtectionDirectoryName, filepath.Base(userWorkingDirectory(userID))+".json")
}

func loadDataProtectionState(userID string) (dataProtectionState, error) {
	content, err := os.ReadFile(dataProtectionStatePath(userID))
	if err != nil {
		return dataProtectionState{}, err
	}
	state, err := json.UnmarshalExtended[dataProtectionState](content)
	if err != nil {
		return dataProtectionState{}, err
	}
	return state, nil
}

func saveDataProtectionState(userID string, state dataProtectionState) error {
	content, err := json.Marshal(state)
	if err != nil {
		return err
	}
	statePath := dataProtectionStatePath(userID)
	err = os.MkdirAll(filepath.Dir(statePath), 0o700)
	if err != nil {
		return E.Cause(err, "create data protection directory")
	}
	return atomicfile.WriteFile(statePath, content, 0o600)
}
