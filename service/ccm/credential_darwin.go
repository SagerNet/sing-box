//go:build darwin && cgo

package ccm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/keybase/go-keychain"
)

func getKeychainServiceName() string {
	configDirectory := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDirectory == "" {
		return "Claude Code-credentials"
	}

	userInfo, err := getRealUser()
	if err != nil {
		return "Claude Code-credentials"
	}
	defaultConfigDirectory := filepath.Join(userInfo.HomeDir, ".claude")
	if configDirectory == defaultConfigDirectory {
		return "Claude Code-credentials"
	}

	hash := sha256.Sum256([]byte(configDirectory))
	return "Claude Code-credentials-" + hex.EncodeToString(hash[:])[:8]
}

func platformReadCredentials(customPath string) (*oauthCredentials, error) {
	if customPath != "" {
		return readCredentialsFromFile(customPath)
	}

	userInfo, err := getRealUser()
	if err == nil {
		query := keychain.NewItem()
		query.SetSecClass(keychain.SecClassGenericPassword)
		query.SetService(getKeychainServiceName())
		query.SetAccount(userInfo.Username)
		query.SetMatchLimit(keychain.MatchLimitOne)
		query.SetReturnData(true)

		results, err := keychain.QueryItem(query)
		if err == nil && len(results) == 1 {
			var container struct {
				ClaudeAIAuth *oauthCredentials `json:"claudeAiOauth,omitempty"`
			}
			unmarshalErr := json.Unmarshal(results[0].Data, &container)
			if unmarshalErr == nil && container.ClaudeAIAuth != nil {
				return container.ClaudeAIAuth, nil
			}
		}
		if err != nil && err != keychain.ErrorItemNotFound {
			return nil, E.Cause(err, "query keychain")
		}
	}

	defaultPath, err := getDefaultCredentialsPath()
	if err != nil {
		return nil, err
	}
	return readCredentialsFromFile(defaultPath)
}

func platformWriteCredentials(oauthCredentials *oauthCredentials, customPath string) error {
	if customPath != "" {
		return writeCredentialsToFile(oauthCredentials, customPath)
	}

	userInfo, err := getRealUser()
	if err == nil {
		data, err := json.Marshal(map[string]any{"claudeAiOauth": oauthCredentials})
		if err == nil {
			serviceName := getKeychainServiceName()
			item := keychain.NewItem()
			item.SetSecClass(keychain.SecClassGenericPassword)
			item.SetService(serviceName)
			item.SetAccount(userInfo.Username)
			item.SetData(data)
			item.SetAccessible(keychain.AccessibleWhenUnlocked)

			err = keychain.AddItem(item)
			if err == nil {
				return nil
			}

			if err == keychain.ErrorDuplicateItem {
				query := keychain.NewItem()
				query.SetSecClass(keychain.SecClassGenericPassword)
				query.SetService(serviceName)
				query.SetAccount(userInfo.Username)

				updateItem := keychain.NewItem()
				updateItem.SetData(data)

				updateErr := keychain.UpdateItem(query, updateItem)
				if updateErr == nil {
					return nil
				}
			}
		}
	}

	defaultPath, err := getDefaultCredentialsPath()
	if err != nil {
		return err
	}
	return writeCredentialsToFile(oauthCredentials, defaultPath)
}
