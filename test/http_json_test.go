package main

import (
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestHTTPJSONConfig(t *testing.T) {
	serverOptions, err := json.UnmarshalExtendedContext[option.Options](globalCtx, []byte(`{
		"inbounds": [{
			"type": "http",
			"listen": "127.0.0.1",
			"listen_port": `+strconv.Itoa(int(serverPort))+`,
			"users": [{"username": "sekai", "password": "password"}],
			"stream_receive_window": "1 MB"
		}],
		"outbounds": [{"type": "direct"}]
	}`))
	require.NoError(t, err)
	startInstance(t, serverOptions)
	clientOptions, err := json.UnmarshalExtendedContext[option.Options](globalCtx, []byte(`{
		"inbounds": [{
			"type": "mixed",
			"listen": "127.0.0.1",
			"listen_port": `+strconv.Itoa(int(clientPort))+`
		}],
		"outbounds": [{
			"type": "http",
			"server": "127.0.0.1",
			"server_port": `+strconv.Itoa(int(serverPort))+`,
			"username": "sekai",
			"password": "password",
			"path": "/proxy"
		}]
	}`))
	require.NoError(t, err)
	startInstance(t, clientOptions)
	origin := newForwardOrigin(t)
	client := proxyClient(t, clientPort)
	request, err := http.NewRequest(http.MethodGet, origin.url("/hello"), nil)
	require.NoError(t, err)
	request.Header.Set("User-Agent", "")
	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
}
