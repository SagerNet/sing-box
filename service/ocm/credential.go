package ocm

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
	oauth2ClientID           = "app_EMoamEEZ73f0CkXaXp7hrann"
	oauth2TokenURL           = "https://auth.openai.com/oauth/token"
	openaiAPIBaseURL         = "https://api.openai.com"
	chatGPTBackendURL        = "https://chatgpt.com/backend-api/codex"
	tokenRefreshIntervalDays = 8
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
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.Join(codexHome, "auth.json"), nil
	}
	userInfo, err := getRealUser()
	if err != nil {
		return "", err
	}
	return filepath.Join(userInfo.HomeDir, ".codex", "auth.json"), nil
}

func readCredentialsFromFile(path string) (*oauthCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var credentials oauthCredentials
	err = json.Unmarshal(data, &credentials)
	if err != nil {
		return nil, err
	}
	return &credentials, nil
}

func writeCredentialsToFile(credentials *oauthCredentials, path string) error {
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

type oauthCredentials struct {
	APIKey      string     `json:"OPENAI_API_KEY,omitempty"`
	Tokens      *tokenData `json:"tokens,omitempty"`
	LastRefresh *time.Time `json:"last_refresh,omitempty"`
}

type tokenData struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id,omitempty"`
}

func (c *oauthCredentials) isAPIKeyMode() bool {
	return c.APIKey != ""
}

func (c *oauthCredentials) getAccessToken() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	if c.Tokens != nil {
		return c.Tokens.AccessToken
	}
	return ""
}

func (c *oauthCredentials) getAccountID() string {
	if c.Tokens != nil {
		return c.Tokens.AccountID
	}
	return ""
}

func (c *oauthCredentials) needsRefresh() bool {
	if c.APIKey != "" {
		return false
	}
	if c.Tokens == nil || c.Tokens.RefreshToken == "" {
		return false
	}
	if c.LastRefresh == nil {
		return true
	}
	return time.Since(*c.LastRefresh) >= time.Duration(tokenRefreshIntervalDays)*24*time.Hour
}

func refreshToken(httpClient *http.Client, credentials *oauthCredentials) (*oauthCredentials, error) {
	if credentials.Tokens == nil || credentials.Tokens.RefreshToken == "" {
		return nil, E.New("refresh token is empty")
	}

	requestBody, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": credentials.Tokens.RefreshToken,
		"client_id":     oauth2ClientID,
		"scope":         "openid profile email",
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
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	err = json.NewDecoder(response.Body).Decode(&tokenResponse)
	if err != nil {
		return nil, E.Cause(err, "decode response")
	}

	newCredentials := *credentials
	if newCredentials.Tokens == nil {
		newCredentials.Tokens = &tokenData{}
	}
	if tokenResponse.IDToken != "" {
		newCredentials.Tokens.IDToken = tokenResponse.IDToken
	}
	if tokenResponse.AccessToken != "" {
		newCredentials.Tokens.AccessToken = tokenResponse.AccessToken
	}
	if tokenResponse.RefreshToken != "" {
		newCredentials.Tokens.RefreshToken = tokenResponse.RefreshToken
	}
	now := time.Now()
	newCredentials.LastRefresh = &now

	return &newCredentials, nil
}
