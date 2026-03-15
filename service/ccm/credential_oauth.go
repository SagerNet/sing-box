package ccm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	oauth2ClientID          = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauth2TokenURL          = "https://platform.claude.com/v1/oauth/token"
	claudeAPIBaseURL        = "https://api.anthropic.com"
	tokenRefreshBufferMs    = 60000
	anthropicBetaOAuthValue = "oauth-2025-04-20"
)

const ccmUserAgentFallback = "claude-code/2.1.72"

var (
	ccmUserAgentOnce  sync.Once
	ccmUserAgentValue string
)

func initCCMUserAgent(logger log.ContextLogger) {
	ccmUserAgentOnce.Do(func() {
		version, err := detectClaudeCodeVersion()
		if err != nil {
			logger.Error("detect Claude Code version: ", err)
			ccmUserAgentValue = ccmUserAgentFallback
			return
		}
		logger.Debug("detected Claude Code version: ", version)
		ccmUserAgentValue = "claude-code/" + version
	})
}

func detectClaudeCodeVersion() (string, error) {
	userInfo, err := getRealUser()
	if err != nil {
		return "", E.Cause(err, "get user")
	}
	binaryName := "claude"
	if runtime.GOOS == "windows" {
		binaryName = "claude.exe"
	}
	linkPath := filepath.Join(userInfo.HomeDir, ".local", "bin", binaryName)
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "", E.Cause(err, "readlink ", linkPath)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	parent := filepath.Base(filepath.Dir(target))
	if parent != "versions" {
		return "", E.New("unexpected symlink target: ", target)
	}
	return filepath.Base(target), nil
}

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

func checkCredentialFileWritable(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return file.Close()
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
	RateLimitTier    string   `json:"rateLimitTier,omitempty"`
	IsMax            bool     `json:"isMax,omitempty"`
}

func (c *oauthCredentials) needsRefresh() bool {
	if c.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixMilli() >= c.ExpiresAt-tokenRefreshBufferMs
}

func refreshToken(ctx context.Context, httpClient *http.Client, credentials *oauthCredentials) (*oauthCredentials, error) {
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

	response, err := doHTTPWithRetry(ctx, httpClient, func() (*http.Request, error) {
		request, err := http.NewRequest("POST", oauth2TokenURL, bytes.NewReader(requestBody))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", ccmUserAgentValue)
		return request, nil
	})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(response.Body)
		return nil, E.New("refresh rate limited: ", response.Status, " ", string(body))
	}
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

func cloneCredentials(credentials *oauthCredentials) *oauthCredentials {
	if credentials == nil {
		return nil
	}
	cloned := *credentials
	cloned.Scopes = append([]string(nil), credentials.Scopes...)
	return &cloned
}

func credentialsEqual(left *oauthCredentials, right *oauthCredentials) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.AccessToken == right.AccessToken &&
		left.RefreshToken == right.RefreshToken &&
		left.ExpiresAt == right.ExpiresAt &&
		slices.Equal(left.Scopes, right.Scopes) &&
		left.SubscriptionType == right.SubscriptionType &&
		left.RateLimitTier == right.RateLimitTier &&
		left.IsMax == right.IsMax
}
