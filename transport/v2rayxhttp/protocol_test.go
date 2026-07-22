package v2rayxhttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2/hpack"
)

func TestResolvedMode(t *testing.T) {
	require.Equal(t, "packet-up", resolvedMode("auto", false, false))
	require.Equal(t, "stream-one", resolvedMode("auto", true, false))
	require.Equal(t, "stream-up", resolvedMode("auto", true, true))
	require.Equal(t, "stream-up", resolvedMode("stream-up", true, false))
}

func TestDownloadSettingsJSON(t *testing.T) {
	options := option.V2RayXHTTPOptions{
		Path: "/upload",
		DownloadSettings: &option.V2RayXHTTPDownloadSettings{
			ServerOptions:               option.ServerOptions{Server: "download.example", ServerPort: 443},
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "download.example"}},
			V2RayXHTTPOptions:           option.V2RayXHTTPOptions{Path: "/download", Mode: "packet-up"},
		},
	}
	encoded, err := json.Marshal(options)
	require.NoError(t, err)
	var decoded option.V2RayXHTTPOptions
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.NotNil(t, decoded.DownloadSettings)
	require.Equal(t, "download.example", decoded.DownloadSettings.Server)
	require.Equal(t, uint16(443), decoded.DownloadSettings.ServerPort)
	require.Equal(t, "/download", decoded.DownloadSettings.Path)
	require.True(t, decoded.DownloadSettings.TLS.Enabled)
}

func TestPacketPayloadChunking(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 600)
	for _, placement := range []string{placementHeader, placementCookie} {
		t.Run(placement, func(t *testing.T) {
			config, err := newConfig(option.V2RayXHTTPOptions{
				UplinkDataPlacement: placement,
				UplinkChunkSize:     option.V2RayXHTTPRange{From: 65, To: 65},
			})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "http://example.com/xhttp/", nil)
			config.fillPacketRequest(request, "session", 0, payload)
			decoded, err := config.extractPacketPayload(request)
			require.NoError(t, err)
			require.Equal(t, payload, decoded)
			for index := 0; ; index++ {
				var chunk string
				if placement == placementHeader {
					chunk = request.Header.Get(config.dataKey + "-" + strconv.Itoa(index))
				} else if cookie, _ := request.Cookie(config.dataKey + "_" + strconv.Itoa(index)); cookie != nil {
					chunk = cookie.Value
				}
				if chunk == "" {
					break
				}
				require.LessOrEqual(t, len(chunk), 65)
			}
		})
	}
}

func TestResponsePadding(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		config, err := newConfig(option.V2RayXHTTPOptions{
			XPaddingBytes: option.V2RayXHTTPRange{From: 12, To: 12},
		})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		config.applyResponsePadding(recorder)
		require.Len(t, recorder.Header().Get("X-Padding"), 12)
	})
	t.Run("obfs-header", func(t *testing.T) {
		config, err := newConfig(option.V2RayXHTTPOptions{
			XPaddingBytes:     option.V2RayXHTTPRange{From: 12, To: 12},
			XPaddingObfsMode:  true,
			XPaddingPlacement: placementHeader,
			XPaddingHeader:    "X-Test-Padding",
			XPaddingMethod:    "tokenish",
		})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		config.applyResponsePadding(recorder)
		value := recorder.Header().Get("X-Test-Padding")
		require.NotEmpty(t, value)
		require.InDelta(t, 12, hpack.HuffmanEncodeLength(value), 2)
	})
}
