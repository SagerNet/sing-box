package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

func TestHTTPSelf(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeHTTP,
				Tag:  "http-out",
				Options: &option.HTTPOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,

							RouteOptions: option.RouteActionOptions{
								Outbound: "http-out",
							},
						},
					},
				},
			},
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestHTTPProxyAuthRetryAfter407(t *testing.T) {
	startHTTPProxyAuthSelfInstance(t)

	err := testPingPongWithConn(t, testPort, func() (net.Conn, error) {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", clientPort))
		if err != nil {
			return nil, err
		}

		reader := bufio.NewReader(conn)
		target := fmt.Sprintf("127.0.0.1:%d", testPort)

		statusCode, err := sendConnectRequest(conn, reader, target, "")
		if err != nil {
			conn.Close()
			return nil, err
		}
		if statusCode != 407 {
			conn.Close()
			return nil, fmt.Errorf("unexpected first CONNECT status code: %d", statusCode)
		}

		authorization := base64.StdEncoding.EncodeToString([]byte("basic:password"))
		statusCode, err = sendConnectRequest(conn, reader, target, authorization)
		if err != nil {
			conn.Close()
			return nil, err
		}
		if statusCode != 200 {
			conn.Close()
			return nil, fmt.Errorf("unexpected second CONNECT status code: %d", statusCode)
		}

		return conn, nil
	})
	require.NoError(t, err)
}

func TestHTTPProxyAuthRetryOnlyOnce(t *testing.T) {
	startHTTPProxyAuthSelfInstance(t)

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", clientPort))
	require.NoError(t, err)
	defer conn.Close()

	reader := bufio.NewReader(conn)
	target := fmt.Sprintf("127.0.0.1:%d", testPort)

	statusCode, err := sendConnectRequest(conn, reader, target, "")
	require.NoError(t, err)
	require.Equal(t, 407, statusCode)

	statusCode, err = sendConnectRequest(conn, reader, target, "")
	require.NoError(t, err)
	require.Equal(t, 407, statusCode)

	authorization := base64.StdEncoding.EncodeToString([]byte("basic:password"))
	err = writeConnectRequest(conn, target, authorization)
	if err == nil {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err = readHTTPStatusAndHeaders(reader)
		require.Error(t, err)
	}

	require.Error(t, err)
}

func TestHTTPProxyAuthRetryTimeout(t *testing.T) {
	startHTTPProxyAuthSelfInstance(t)

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", clientPort))
	require.NoError(t, err)
	defer conn.Close()

	reader := bufio.NewReader(conn)
	target := fmt.Sprintf("127.0.0.1:%d", testPort)

	statusCode, err := sendConnectRequest(conn, reader, target, "")
	require.NoError(t, err)
	require.Equal(t, 407, statusCode)

	time.Sleep(C.TCPConnectTimeout + time.Second)

	authorization := base64.StdEncoding.EncodeToString([]byte("basic:password"))
	err = writeConnectRequest(conn, target, authorization)
	if err == nil {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, err = readHTTPStatusAndHeaders(reader)
		require.Error(t, err)
	}

	require.Error(t, err)
}

func startHTTPProxyAuthSelfInstance(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
					Users: []auth.User{{
						Username: "basic",
						Password: "password",
					}},
				},
			},
			{
				Type: C.TypeMixed,
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
			{
				Type: C.TypeHTTP,
				Tag:  "http-out",
				Options: &option.HTTPOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"mixed-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "http-out",
							},
						},
					},
				},
			},
		},
	})
}

func sendConnectRequest(conn net.Conn, reader *bufio.Reader, target string, authorization string) (int, error) {
	err := writeConnectRequest(conn, target, authorization)
	if err != nil {
		return 0, err
	}
	return readHTTPStatusAndHeaders(reader)
}

func writeConnectRequest(conn net.Conn, target string, authorization string) error {
	_, err := fmt.Fprint(conn, buildConnectRequest(target, authorization))
	return err
}

func buildConnectRequest(target string, authorization string) string {
	if authorization == "" {
		return fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: keep-alive\r\n\r\n", target, target)
	}
	return fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: keep-alive\r\nProxy-Authorization: Basic %s\r\n\r\n", target, target, authorization)
}

func readHTTPStatusAndHeaders(reader *bufio.Reader) (int, error) {
	tpReader := textproto.NewReader(reader)
	statusLine, err := tpReader.ReadLine()
	if err != nil {
		return 0, err
	}
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid status line: %s", statusLine)
	}
	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid status code %q: %w", parts[1], err)
	}
	if _, err = tpReader.ReadMIMEHeader(); err != nil {
		return 0, err
	}
	return statusCode, nil
}
