package warp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	apiBase string = "https://api.cloudflareclient.com/v0a4005"
)

var client = makeClient()

func defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type":      "application/json; charset=UTF-8",
		"User-Agent":        "okhttp/3.12.1",
		"CF-Client-Version": "a-6.30-3596",
	}
}

func makeClient() *http.Client {
	// Create a custom dialer using the TLS config
	plainDialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 5 * time.Second,
	}
	tlsDialer := Dialer{}
	// Create a custom HTTP transport
	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return tlsDialer.TLSDial(plainDialer, network, addr)
		},
	}

	// Create a custom HTTP client using the transport
	return &http.Client{
		Transport: transport,
		// Other client configurations can be added here
	}
}

type IdentityAccount struct {
	Created                  string `json:"created"`
	Updated                  string `json:"updated"`
	License                  string `json:"license"`
	PremiumData              int64  `json:"premium_data"`
	WarpPlus                 bool   `json:"warp_plus"`
	AccountType              string `json:"account_type"`
	ReferralRenewalCountdown int64  `json:"referral_renewal_countdown"`
	Role                     string `json:"role"`
	ID                       string `json:"id"`
	Quota                    int64  `json:"quota"`
	Usage                    int64  `json:"usage"`
	ReferralCount            int64  `json:"referral_count"`
	TTL                      string `json:"ttl"`
}

type IdentityConfigPeerEndpoint struct {
	V4    string   `json:"v4"`
	V6    string   `json:"v6"`
	Host  string   `json:"host"`
	Ports []uint16 `json:"ports"`
}

type IdentityConfigPeer struct {
	PublicKey string                     `json:"public_key"`
	Endpoint  IdentityConfigPeerEndpoint `json:"endpoint"`
}

type IdentityConfigInterfaceAddresses struct {
	V4 string `json:"v4"`
	V6 string `json:"v6"`
}

type IdentityConfigInterface struct {
	Addresses IdentityConfigInterfaceAddresses `json:"addresses"`
}
type IdentityConfigServices struct {
	HTTPProxy string `json:"http_proxy"`
}

type IdentityConfig struct {
	Peers     []IdentityConfigPeer    `json:"peers"`
	Interface IdentityConfigInterface `json:"interface"`
	Services  IdentityConfigServices  `json:"services"`
	ClientID  string                  `json:"client_id"`
}

type Identity struct {
	PrivateKey      string          `json:"private_key"`
	Key             string          `json:"key"`
	Account         IdentityAccount `json:"account"`
	Place           int64           `json:"place"`
	FCMToken        string          `json:"fcm_token"`
	Name            string          `json:"name"`
	TOS             string          `json:"tos"`
	Locale          string          `json:"locale"`
	InstallID       string          `json:"install_id"`
	WarpEnabled     bool            `json:"warp_enabled"`
	Type            string          `json:"type"`
	Model           string          `json:"model"`
	Config          IdentityConfig  `json:"config"`
	Token           string          `json:"token"`
	Enabled         bool            `json:"enabled"`
	ID              string          `json:"id"`
	Created         string          `json:"created"`
	Updated         string          `json:"updated"`
	WaitlistEnabled bool            `json:"waitlist_enabled"`
}

type IdentityDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Model     string `json:"model"`
	Created   string `json:"created"`
	Activated string `json:"updated"`
	Active    bool   `json:"active"`
	Role      string `json:"role"`
}

type License struct {
	License string `json:"license"`
}

func GetAccount(authToken, deviceID string) (IdentityAccount, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s/account", apiBase, deviceID)
	method := "GET"

	req, err := http.NewRequest(method, reqUrl, nil)
	if err != nil {
		return IdentityAccount{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return IdentityAccount{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return IdentityAccount{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return IdentityAccount{}, err
	}

	var rspData = IdentityAccount{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return IdentityAccount{}, err
	}

	return rspData, nil
}

func GetBoundDevices(authToken, deviceID string) ([]IdentityDevice, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s/account/devices", apiBase, deviceID)
	method := "GET"

	req, err := http.NewRequest(method, reqUrl, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rspData = []IdentityDevice{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return nil, err
	}

	return rspData, nil
}

func GetSourceDevice(authToken, deviceID string) (Identity, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s", apiBase, deviceID)
	method := "GET"

	req, err := http.NewRequest(method, reqUrl, nil)
	if err != nil {
		return Identity{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Identity{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Identity{}, err
	}

	var rspData = Identity{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return Identity{}, err
	}

	return rspData, nil
}

func Register(publicKey string) (Identity, error) {
	reqUrl := fmt.Sprintf("%s/reg", apiBase)
	method := "POST"

	data := map[string]interface{}{
		"install_id":   "",
		"fcm_token":    "",
		"tos":          time.Now().Format(time.RFC3339Nano),
		"key":          publicKey,
		"type":         "Android",
		"model":        "PC",
		"locale":       "en_US",
		"warp_enabled": true,
	}

	jsonBody, err := json.Marshal(data)
	if err != nil {
		return Identity{}, err
	}

	req, err := http.NewRequest(method, reqUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return Identity{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Identity{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Identity{}, err
	}

	var rspData = Identity{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return Identity{}, err
	}

	return rspData, nil
}

func ResetAccountLicense(authToken, deviceID string) (License, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s/account/license", apiBase, deviceID)
	method := "POST"

	req, err := http.NewRequest(method, reqUrl, nil)
	if err != nil {
		return License{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return License{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return License{}, fmt.Errorf("API request failed with response: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return License{}, err
	}

	var rspData = License{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return License{}, err
	}

	return rspData, nil
}

func UpdateAccount(authToken, deviceID, license string) (IdentityAccount, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s/account", apiBase, deviceID)
	method := "PUT"

	jsonBody, err := json.Marshal(map[string]interface{}{"license": license})
	if err != nil {
		return IdentityAccount{}, err
	}

	req, err := http.NewRequest(method, reqUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return IdentityAccount{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return IdentityAccount{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return IdentityAccount{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return IdentityAccount{}, err
	}

	var rspData = IdentityAccount{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return IdentityAccount{}, err
	}

	return rspData, nil
}

func UpdateBoundDevice(authToken, deviceID, otherDeviceID, name string, active bool) (IdentityDevice, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s/account/reg/%s", apiBase, deviceID, otherDeviceID)
	method := "PATCH"

	data := map[string]interface{}{
		"active": active,
		"name":   name,
	}

	jsonBody, err := json.Marshal(data)
	if err != nil {
		return IdentityDevice{}, err
	}

	req, err := http.NewRequest(method, reqUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return IdentityDevice{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return IdentityDevice{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return IdentityDevice{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return IdentityDevice{}, err
	}

	var rspData = IdentityDevice{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return IdentityDevice{}, err
	}

	return rspData, nil
}

func UpdateSourceDevice(authToken, deviceID, publicKey string) (Identity, error) {
	reqUrl := fmt.Sprintf("%s/reg/%s", apiBase, deviceID)
	method := "PATCH"

	jsonBody, err := json.Marshal(map[string]interface{}{"key": publicKey})
	if err != nil {
		return Identity{}, err
	}

	req, err := http.NewRequest(method, reqUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return Identity{}, err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Identity{}, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	// convert response to byte array
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Identity{}, err
	}

	var rspData = Identity{}
	if err := json.Unmarshal(responseData, &rspData); err != nil {
		return Identity{}, err
	}

	return rspData, nil
}

func DeleteDevice(authToken, deviceID string) error {
	reqUrl := fmt.Sprintf("%s/reg/%s", apiBase, deviceID)
	method := "DELETE"

	req, err := http.NewRequest(method, reqUrl, nil)
	if err != nil {
		return err
	}

	// Set headers
	for k, v := range defaultHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Create HTTP client and execute request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}
