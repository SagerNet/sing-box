package ccm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	oauth2ClientID          = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauth2TokenURL          = "https://console.anthropic.com/v1/oauth/token"
	claudeAPIBaseURL        = "https://api.anthropic.com"
	tokenRefreshBufferMs    = 60000
	anthropicBetaOAuthValue = "oauth-2025-04-20"
)

func getRealUser() (*user.User, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		sudoUserInfo, err := user.Lookup(sudoUser)
		if err == nil {
			return sudoUserInfo, nil
		}
	}
	return user.Current()
}

func getDefaultCredentialsPath() (string, error) {
	if configDir := os.Getenv("CLAUDE_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, ".credentials.json"), nil
	}
	userInfo, err := getRealUser()
	if err != nil {
		return "", err
	}
	return filepath.Join(userInfo.HomeDir, ".claude", ".credentials.json"), nil
}

func readCredentialsFromFile(path string) (*oauthCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var credentialsContainer struct {
		ClaudeAIAuth *oauthCredentials `json:"claudeAiOauth,omitempty"`
	}
	err = json.Unmarshal(data, &credentialsContainer)
	if err != nil {
		return nil, err
	}
	if credentialsContainer.ClaudeAIAuth == nil {
		return nil, E.New("claudeAiOauth field not found in credentials")
	}
	return credentialsContainer.ClaudeAIAuth, nil
}

func writeCredentialsToFile(oauthCredentials *oauthCredentials, path string) error {
	data, err := json.MarshalIndent(map[string]any{
		"claudeAiOauth": oauthCredentials,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type oauthCredentials struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"`
	Scopes           []string `json:"scopes,omitempty"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
	IsMax            bool     `json:"isMax,omitempty"`
}

func (c *oauthCredentials) needsRefresh() bool {
	if c.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixMilli() >= c.ExpiresAt-tokenRefreshBufferMs
}

func refreshToken(httpClient *http.Client, credentials *oauthCredentials) (*oauthCredentials, error) {
	if credentials.RefreshToken == "" {
		return nil, E.New("refresh token is empty")
	}

	requestBody, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": credentials.RefreshToken,
		"client_id":     oauth2ClientID,
	})
	if err != nil {
		return nil, E.Cause(err, "marshal request")
	}

	request, err := http.NewRequest("POST", oauth2TokenURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, E.New("refresh failed: ", response.Status, " ", string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	err = json.NewDecoder(response.Body).Decode(&tokenResponse)
	if err != nil {
		return nil, E.Cause(err, "decode response")
	}

	newCredentials := *credentials
	newCredentials.AccessToken = tokenResponse.AccessToken
	if tokenResponse.RefreshToken != "" {
		newCredentials.RefreshToken = tokenResponse.RefreshToken
	}
	newCredentials.ExpiresAt = time.Now().UnixMilli() + int64(tokenResponse.ExpiresIn)*1000

	return &newCredentials, nil
}
