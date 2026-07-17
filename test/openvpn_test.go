package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	typesapi "github.com/docker/docker/api/types"
	containerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

const (
	openVPNTLSUsername             = "test-user"
	openVPNTLSPassword             = "test-password"
	openVPNStaticChallengeText     = "Enter OTP"
	openVPNStaticChallengeResponse = "31337"
	openVPNLargeDataTimeout        = 3 * time.Minute
	openVPNLargeTCPPackets         = 1
	openVPNLargeTCPSize            = 2048

	openVPNDockerImage          = "sing-box-openvpn-2.6.14-options:test"
	openVPNDockerPackageVersion = "2.6.14-0+deb12u2"
	openVPNDockerRoot           = "/config"
)

var (
	openVPNDockerImageOnce sync.Once
	openVPNDockerImageErr  error
)

type openVPNCertificateBundle struct {
	caPath         string
	serverCertPath string
	serverKeyPath  string
	clientCertPath string
	clientKeyPath  string
}

type openVPNDockerServerEnvironment struct {
	certificates openVPNCertificateBundle
	workspace    string
	openVPNPort  uint16
	echoPort     uint16
	container    *openVPNDockerContainer
}

type openVPNSelfCase struct {
	name     string
	protocol string
	tlsCrypt bool
}

func TestOpenVPNSelfToSelf(t *testing.T) {
	testCases := []openVPNSelfCase{
		{
			name:     "real_tls_udp",
			protocol: N.NetworkUDP,
		},
		{
			name:     "real_tls_tcp",
			protocol: N.NetworkTCP,
		},
		{
			name:     "real_tls_udp_tls_crypt",
			protocol: N.NetworkUDP,
			tlsCrypt: true,
		},
	}
	for i := range testCases {
		currentTestCase := testCases[i]
		t.Run(currentTestCase.name, func(t *testing.T) {
			runOpenVPNSelfToSelf(t, currentTestCase)
		})
	}
}

func TestOpenVPNDockerInterop(t *testing.T) {
	t.Run("official_server_to_sing_box_client", func(t *testing.T) {
		testOpenVPNDockerOfficialServerToSingBoxClient(t)
	})
	t.Run("official_client_to_sing_box_server", func(t *testing.T) {
		testOpenVPNDockerOfficialClientToSingBoxServer(t)
	})
}

func runOpenVPNSelfToSelf(t *testing.T, testCase openVPNSelfCase) {
	t.Helper()
	const serverAddress = "10.8.0.1"
	serverPrefix := netip.MustParsePrefix(serverAddress + "/24")
	proxyPort := reserveOpenVPNTCPPort(t)
	openVPNPort := reserveOpenVPNProtocolPort(t, testCase.protocol)
	echoPort := reserveOpenVPNEchoPort(t)
	readinessPort := reserveOpenVPNEchoPort(t)
	certificates := createOpenVPNCertificateBundle(t)
	serverOptions := option.OpenVPNServerEndpointOptions{
		ListenOptions: option.ListenOptions{
			Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
			ListenPort: openVPNPort,
		},
		Network: testCase.protocol,
		Address: []netip.Prefix{serverPrefix},
		Users: []auth.User{
			{
				Username: openVPNTLSUsername,
				Password: openVPNTLSPassword,
			},
		},
		TLS: &option.OpenVPNInboundTLSOptions{
			CertificatePath:       certificates.serverCertPath,
			KeyPath:               certificates.serverKeyPath,
			ClientCertificatePath: certificates.caPath,
		},
	}
	clientOptions := newOpenVPNTLSClientOptions(testCase.protocol, openVPNPort, certificates.caPath, certificates.clientCertPath, certificates.clientKeyPath)
	if testCase.tlsCrypt {
		tlsCryptKeyPath := writeOpenVPNStaticKeyFile(t, createOpenVPNStaticKey(t))
		serverOptions.TLS.ControlWrap = &option.OpenVPNControlWrapOptions{
			Type:    "tls_crypt",
			KeyPath: tlsCryptKeyPath,
		}
		clientOptions.TLS.ControlWrap = &option.OpenVPNControlWrapOptions{
			Type:    "tls_crypt",
			KeyPath: tlsCryptKeyPath,
		}
	}
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword

	startInstance(t, openVPNServerInstanceOptions(serverOptions))
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNClientReady(t, proxyPort, readinessPort, serverAddress)
	testSuitOpenVPN(t, proxyPort, echoPort, serverAddress)
}

func testOpenVPNDockerOfficialServerToSingBoxClient(t *testing.T) {
	t.Helper()
	environment := startOpenVPNDockerOfficialServer(t, "official-server")
	proxyPort := reserveOpenVPNTCPPort(t)

	clientOptions := newOpenVPNTLSClientOptions(
		N.NetworkUDP,
		environment.openVPNPort,
		filepath.Join(environment.workspace, "ca.crt"),
		filepath.Join(environment.workspace, "client.crt"),
		filepath.Join(environment.workspace, "client.key"),
	)
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNRemoteReady(t, proxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
	testRemoteEchoThroughSocks(t, proxyPort, "10.8.0.1", environment.echoPort)
}

func TestOpenVPNDockerMultiRemoteFailover(t *testing.T) {
	environment := startOpenVPNDockerOfficialServer(t, "multi-remote-server")
	deadPort := reserveOpenVPNTCPPort(t)
	proxyPort := reserveOpenVPNTCPPort(t)
	clientOptions := newOpenVPNTLSClientOptions(
		N.NetworkUDP,
		environment.openVPNPort,
		filepath.Join(environment.workspace, "ca.crt"),
		filepath.Join(environment.workspace, "client.crt"),
		filepath.Join(environment.workspace, "client.key"),
	)
	clientOptions.Server = ""
	clientOptions.ServerPort = 0
	clientOptions.Servers = []option.OpenVPNRemoteOptions{
		{
			ServerOptions: option.ServerOptions{
				Server:     "127.0.0.1",
				ServerPort: deadPort,
			},
			Network: N.NetworkTCP,
		},
		{
			ServerOptions: option.ServerOptions{
				Server:     "127.0.0.1",
				ServerPort: environment.openVPNPort,
			},
			Network: N.NetworkUDP,
		},
	}
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNRemoteReady(t, proxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
	testRemoteEchoThroughSocks(t, proxyPort, "10.8.0.1", environment.echoPort)
}

func TestOpenVPNDockerMSSFix(t *testing.T) {
	dockerClient := requireOpenVPNDockerEnvironment(t)
	certificates := createOpenVPNCertificateBundle(t)
	workspace := newOpenVPNDockerWorkspace(t, certificates)
	openVPNPort := reserveOpenVPNUDPPort(t)
	proxyPort := reserveOpenVPNTCPPort(t)
	echoPort := reserveOpenVPNEchoPort(t)
	writeOpenVPNDockerServerConfig(t, workspace, openVPNPort, "/config/check_userpass.sh", "mssfix 1200")
	writeOpenVPNDockerEchoServer(t, workspace, "10.8.0.1", echoPort)
	serverCommand := strings.Join([]string{
		"python3 /config/echo_server.py &",
		"openvpn --config /config/server.conf &",
		"openvpn_pid=$!",
		"until grep -q 'Initialization Sequence Completed' /config/openvpn.log; do",
		"  if ! kill -0 \"$openvpn_pid\" 2>/dev/null; then cat /config/openvpn.log; exit 1; fi",
		"  sleep 0.1",
		"done",
		"tcpdump -i tun0 -l -nn -v 'dst host 10.8.0.1 and tcp[tcpflags] & tcp-syn != 0' > /config/mss.log 2>&1 &",
		"wait \"$openvpn_pid\"",
	}, "\n")
	serverContainer := startOpenVPNDockerContainer(t, dockerClient, "mss-fix-server", workspace, serverCommand)
	dumpOpenVPNDockerLogsOnFailure(t, serverContainer, workspace)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "openvpn.log"), "Initialization Sequence Completed", 30*time.Second)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "echo.ready"), "ready", 30*time.Second)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "mss.log"), "listening on tun0", 30*time.Second)

	clientOptions := newOpenVPNTLSClientOptions(
		N.NetworkUDP,
		openVPNPort,
		filepath.Join(workspace, "ca.crt"),
		filepath.Join(workspace, "client.crt"),
		filepath.Join(workspace, "client.key"),
	)
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword
	clientOptions.MSSFix = 1200
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNRemoteReady(t, proxyPort, "10.8.0.1", echoPort, 30*time.Second)
	testRemoteEchoThroughSocks(t, proxyPort, "10.8.0.1", echoPort)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "mss.log"), "mss 1136", 30*time.Second)
}

func TestOpenVPNDockerCompressionNegotiation(t *testing.T) {
	environment := startOpenVPNDockerOfficialServer(t, "compression-server",
		"allow-compression no",
		"compress stub",
		"push \"compress stub\"",
	)
	proxyPort := reserveOpenVPNTCPPort(t)
	clientOptions := newOpenVPNTLSClientOptions(
		N.NetworkUDP,
		environment.openVPNPort,
		filepath.Join(environment.workspace, "ca.crt"),
		filepath.Join(environment.workspace, "client.crt"),
		filepath.Join(environment.workspace, "client.key"),
	)
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword
	clientOptions.Compression = "stub"
	clientOptions.AllowCompression = "asym"
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNRemoteReady(t, proxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
	testRemoteEchoThroughSocks(t, proxyPort, "10.8.0.1", environment.echoPort)
	waitForOpenVPNDockerFile(t, environment.container, filepath.Join(environment.workspace, "openvpn.log"), "IV_LZO=1", 30*time.Second)
}

func TestOpenVPNDockerPeerFingerprint(t *testing.T) {
	environment := startOpenVPNDockerOfficialServer(t, "peer-fingerprint-server")
	fingerprint := openVPNCertificateSHA256Fingerprint(t, environment.certificates.serverCertPath)
	wrongFingerprint := "0" + fingerprint[1:]
	if fingerprint[0] == '0' {
		wrongFingerprint = "1" + fingerprint[1:]
	}

	t.Run("matching", func(matchingTest *testing.T) {
		matchingProxyPort := reserveOpenVPNTCPPort(matchingTest)
		matchingClientOptions := newOpenVPNTLSClientOptions(
			N.NetworkUDP,
			environment.openVPNPort,
			"",
			filepath.Join(environment.workspace, "client.crt"),
			filepath.Join(environment.workspace, "client.key"),
		)
		matchingClientOptions.Username = openVPNTLSUsername
		matchingClientOptions.Password = openVPNTLSPassword
		matchingClientOptions.TLS.PeerFingerprint = []string{fingerprint}
		startInstance(matchingTest, openVPNClientInstanceOptions(matchingClientOptions, matchingProxyPort))
		waitForOpenVPNRemoteReady(matchingTest, matchingProxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
		testRemoteEchoThroughSocks(matchingTest, matchingProxyPort, "10.8.0.1", environment.echoPort)
	})

	t.Run("mismatch", func(mismatchTest *testing.T) {
		mismatchProxyPort := reserveOpenVPNTCPPort(mismatchTest)
		mismatchClientOptions := newOpenVPNTLSClientOptions(
			N.NetworkUDP,
			environment.openVPNPort,
			"",
			filepath.Join(environment.workspace, "client.crt"),
			filepath.Join(environment.workspace, "client.key"),
		)
		mismatchClientOptions.Username = openVPNTLSUsername
		mismatchClientOptions.Password = openVPNTLSPassword
		mismatchClientOptions.TLS.PeerFingerprint = []string{wrongFingerprint}
		mismatchInstance := startInstance(mismatchTest, openVPNClientInstanceOptions(mismatchClientOptions, mismatchProxyPort))
		mismatchEndpoint := requireOpenVPNEndpoint(mismatchTest, mismatchInstance, "openvpn-client")
		mismatchStatus := waitForOpenVPNStatus(mismatchTest, mismatchEndpoint, 30*time.Second, func(status adapter.OpenVPNStatus) bool {
			return status.State == adapter.OpenVPNStateError
		})
		require.Contains(mismatchTest, mismatchStatus.Error, "peer fingerprint mismatch")
	})
}

func TestOpenVPNDockerClientRoutes(t *testing.T) {
	environment := startOpenVPNDockerOfficialServer(t, "client-routes-server")
	localRoute := netip.MustParsePrefix("192.0.2.0/24")

	t.Run("routes", func(routesTest *testing.T) {
		routesProxyPort := reserveOpenVPNTCPPort(routesTest)
		routesClientOptions := newOpenVPNTLSClientOptions(
			N.NetworkUDP,
			environment.openVPNPort,
			filepath.Join(environment.workspace, "ca.crt"),
			filepath.Join(environment.workspace, "client.crt"),
			filepath.Join(environment.workspace, "client.key"),
		)
		routesClientOptions.Username = openVPNTLSUsername
		routesClientOptions.Password = openVPNTLSPassword
		routesClientOptions.Routes = []netip.Prefix{localRoute}
		routesInstance := startInstance(routesTest, openVPNClientInstanceOptions(routesClientOptions, routesProxyPort))
		waitForOpenVPNRemoteReady(routesTest, routesProxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
		routesEndpoint, loaded := routesInstance.Endpoint().Get("openvpn-client")
		require.True(routesTest, loaded)
		preferredRoutes, supported := routesEndpoint.(adapter.OutboundWithPreferredRoutes)
		require.True(routesTest, supported)
		require.True(routesTest, preferredRoutes.PreferredAddress(nil, netip.MustParseAddr("192.0.2.1")))
		require.False(routesTest, preferredRoutes.PreferredAddress(nil, netip.MustParseAddr("198.51.100.1")))
	})

	t.Run("redirect_gateway", func(redirectTest *testing.T) {
		redirectProxyPort := reserveOpenVPNTCPPort(redirectTest)
		redirectClientOptions := newOpenVPNTLSClientOptions(
			N.NetworkUDP,
			environment.openVPNPort,
			filepath.Join(environment.workspace, "ca.crt"),
			filepath.Join(environment.workspace, "client.crt"),
			filepath.Join(environment.workspace, "client.key"),
		)
		redirectClientOptions.Username = openVPNTLSUsername
		redirectClientOptions.Password = openVPNTLSPassword
		redirectClientOptions.Routes = []netip.Prefix{localRoute}
		redirectClientOptions.RedirectGateway = true
		redirectClientOptions.RedirectGatewayFlags = []string{"def1"}
		redirectInstance := startInstance(redirectTest, openVPNClientInstanceOptions(redirectClientOptions, redirectProxyPort))
		waitForOpenVPNRemoteReady(redirectTest, redirectProxyPort, "10.8.0.1", environment.echoPort, 30*time.Second)
		redirectEndpoint, loaded := redirectInstance.Endpoint().Get("openvpn-client")
		require.True(redirectTest, loaded)
		preferredRoutes, supported := redirectEndpoint.(adapter.OutboundWithPreferredRoutes)
		require.True(redirectTest, supported)
		require.True(redirectTest, preferredRoutes.PreferredAddress(nil, netip.MustParseAddr("192.0.2.1")))
		require.True(redirectTest, preferredRoutes.PreferredAddress(nil, netip.MustParseAddr("198.51.100.1")))
		require.False(redirectTest, preferredRoutes.PreferredAddress(nil, netip.MustParseAddr("2001:db8::1")))
	})
}

func startOpenVPNDockerOfficialServer(t *testing.T, nameSuffix string, serverDirectives ...string) openVPNDockerServerEnvironment {
	t.Helper()
	dockerClient := requireOpenVPNDockerEnvironment(t)
	certificates := createOpenVPNCertificateBundle(t)
	workspace := newOpenVPNDockerWorkspace(t, certificates)
	openVPNPort := reserveOpenVPNUDPPort(t)
	echoPort := reserveOpenVPNEchoPort(t)
	writeOpenVPNDockerServerConfig(t, workspace, openVPNPort, "/config/check_userpass.sh", serverDirectives...)
	writeOpenVPNDockerEchoServer(t, workspace, "10.8.0.1", echoPort)
	serverContainer := startOpenVPNDockerContainer(t, dockerClient, nameSuffix, workspace, "python3 /config/echo_server.py & exec openvpn --config /config/server.conf")
	dumpOpenVPNDockerLogsOnFailure(t, serverContainer, workspace)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "openvpn.log"), "Initialization Sequence Completed", 30*time.Second)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "echo.ready"), "ready", 30*time.Second)
	return openVPNDockerServerEnvironment{
		certificates: certificates,
		workspace:    workspace,
		openVPNPort:  openVPNPort,
		echoPort:     echoPort,
		container:    serverContainer,
	}
}

func TestOpenVPNInteractiveAuth(t *testing.T) {
	dockerClient := requireOpenVPNDockerEnvironment(t)
	certificates := createOpenVPNCertificateBundle(t)
	workspace := newOpenVPNDockerWorkspace(t, certificates)
	openVPNPort := reserveOpenVPNUDPPort(t)
	proxyPort := reserveOpenVPNTCPPort(t)
	echoPort := reserveOpenVPNEchoPort(t)

	writeOpenVPNDockerStaticChallengeScript(t, workspace)
	writeOpenVPNDockerServerConfig(t, workspace, openVPNPort, "/config/check_scrv1.sh")
	writeOpenVPNDockerEchoServer(t, workspace, "10.8.0.1", echoPort)
	serverContainer := startOpenVPNDockerContainer(t, dockerClient, "interactive-auth-server", workspace, "python3 /config/echo_server.py & exec openvpn --config /config/server.conf")
	dumpOpenVPNDockerLogsOnFailure(t, serverContainer, workspace)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "openvpn.log"), "Initialization Sequence Completed", 30*time.Second)
	waitForOpenVPNDockerFile(t, serverContainer, filepath.Join(workspace, "echo.ready"), "ready", 30*time.Second)

	clientOptions := newOpenVPNTLSClientOptions(
		N.NetworkUDP,
		openVPNPort,
		filepath.Join(workspace, "ca.crt"),
		filepath.Join(workspace, "client.crt"),
		filepath.Join(workspace, "client.key"),
	)
	clientOptions.StaticChallenge = openVPNStaticChallengeText
	clientOptions.StaticChallengeEcho = true
	clientInstance := startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	clientEndpoint := requireOpenVPNEndpoint(t, clientInstance, "openvpn-client")

	challengeStatus := waitForOpenVPNStatus(t, clientEndpoint, 30*time.Second, func(status adapter.OpenVPNStatus) bool {
		require.NotEqual(t, adapter.OpenVPNStateConnected, status.State)
		require.NotEqual(t, adapter.OpenVPNStateError, status.State, status.Error)
		return status.State == adapter.OpenVPNStateAuthPending
	})
	challenge := challengeStatus.Challenge
	require.NotNil(t, challenge)
	require.NotEmpty(t, challenge.ID)
	require.Equal(t, "credentials", challenge.Kind)
	require.Equal(t, openVPNStaticChallengeText, challenge.SecretMessage)
	require.True(t, challenge.Echo)
	require.Empty(t, challenge.PreviousError)

	err := clientEndpoint.CompleteChallenge(challenge.ID, adapter.OpenVPNChallengeResponse{
		Username: openVPNTLSUsername,
		Password: openVPNTLSPassword,
		Secret:   openVPNStaticChallengeResponse,
	})
	require.NoError(t, err)

	waitForOpenVPNStatus(t, clientEndpoint, time.Minute, func(status adapter.OpenVPNStatus) bool {
		require.NotEqual(t, adapter.OpenVPNStateError, status.State, status.Error)
		return status.State == adapter.OpenVPNStateConnected
	})
	waitForOpenVPNRemoteReady(t, proxyPort, "10.8.0.1", echoPort, 30*time.Second)
	testRemoteEchoThroughSocks(t, proxyPort, "10.8.0.1", echoPort)
}

func writeOpenVPNDockerStaticChallengeScript(t *testing.T, workspace string) {
	t.Helper()
	script := strings.Join([]string{
		"#!/bin/bash",
		"set -eu",
		"credentials_file=\"$1\"",
		"username=\"$(sed -n '1p' \"$credentials_file\")\"",
		"password=\"$(sed -n '2p' \"$credentials_file\")\"",
		"[ \"$username\" = \"" + openVPNTLSUsername + "\" ] || exit 1",
		"case \"$password\" in",
		"SCRV1:*) ;;",
		"*) exit 1 ;;",
		"esac",
		"encoded_password=\"$(printf '%s' \"$password\" | cut -d: -f2)\"",
		"encoded_response=\"$(printf '%s' \"$password\" | cut -d: -f3)\"",
		"[ \"$(printf '%s' \"$encoded_password\" | base64 -d)\" = \"" + openVPNTLSPassword + "\" ] || exit 1",
		"[ \"$(printf '%s' \"$encoded_response\" | base64 -d)\" = \"" + openVPNStaticChallengeResponse + "\" ] || exit 1",
		"exit 0",
		"",
	}, "\n")
	err := os.WriteFile(filepath.Join(workspace, "check_scrv1.sh"), []byte(script), 0o700)
	require.NoError(t, err)
}

func requireOpenVPNEndpoint(t *testing.T, instance *box.Box, tag string) adapter.OpenVPNEndpoint {
	t.Helper()
	endpoint, loaded := instance.Endpoint().Get(tag)
	require.True(t, loaded)
	openVPNEndpoint, supported := endpoint.(adapter.OpenVPNEndpoint)
	require.True(t, supported)
	return openVPNEndpoint
}

func waitForOpenVPNStatus(t *testing.T, endpoint adapter.OpenVPNEndpoint, timeout time.Duration, predicate func(status adapter.OpenVPNStatus) bool) adapter.OpenVPNStatus {
	t.Helper()
	timeoutChannel := time.After(timeout)
	for {
		statusUpdated := endpoint.StatusUpdated()
		status := endpoint.OpenVPNStatus()
		if predicate(status) {
			return status
		}
		select {
		case <-statusUpdated:
		case <-timeoutChannel:
			t.Fatalf("timed out waiting for OpenVPN endpoint status, last state %q, error %q", status.State, status.Error)
		}
	}
}

func TestOpenVPNClientReconnectSelfToSelf(t *testing.T) {
	const serverAddress = "10.8.0.1"
	serverPrefix := netip.MustParsePrefix(serverAddress + "/24")
	proxyPort := reserveOpenVPNTCPPort(t)
	openVPNPort := reserveOpenVPNUDPPort(t)
	echoPort := reserveOpenVPNEchoPort(t)
	readinessPort := reserveOpenVPNEchoPort(t)
	certificates := createOpenVPNCertificateBundle(t)
	serverOptions := option.OpenVPNServerEndpointOptions{
		ListenOptions: option.ListenOptions{
			Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
			ListenPort: openVPNPort,
		},
		Network: N.NetworkUDP,
		Address: []netip.Prefix{serverPrefix},
		TLS: &option.OpenVPNInboundTLSOptions{
			CertificatePath:       certificates.serverCertPath,
			KeyPath:               certificates.serverKeyPath,
			ClientCertificatePath: certificates.caPath,
		},
		KeepaliveInterval: badoption.Duration(time.Second),
		KeepaliveTimeout:  badoption.Duration(2 * time.Second),
		Users: []auth.User{
			{
				Username: openVPNTLSUsername,
				Password: openVPNTLSPassword,
			},
		},
	}
	clientOptions := newOpenVPNTLSClientOptions(N.NetworkUDP, openVPNPort, certificates.caPath, certificates.clientCertPath, certificates.clientKeyPath)
	clientOptions.Username = openVPNTLSUsername
	clientOptions.Password = openVPNTLSPassword
	serverInstance := startInstance(t, openVPNServerInstanceOptions(serverOptions))
	startInstance(t, openVPNClientInstanceOptions(clientOptions, proxyPort))
	waitForOpenVPNClientReady(t, proxyPort, readinessPort, serverAddress)
	err := serverInstance.Close()
	require.NoError(t, err)
	startInstance(t, openVPNServerInstanceOptions(serverOptions))
	waitForOpenVPNClientReady(t, proxyPort, readinessPort, serverAddress)
	testSuitOpenVPN(t, proxyPort, echoPort, serverAddress)
}

func testOpenVPNDockerOfficialClientToSingBoxServer(t *testing.T) {
	t.Helper()
	dockerClient := requireOpenVPNDockerEnvironment(t)
	certificates := createOpenVPNCertificateBundle(t)
	workspace := newOpenVPNDockerWorkspace(t, certificates)
	openVPNPort := reserveOpenVPNUDPPort(t)
	echoPort := reserveOpenVPNEchoPort(t)

	startOpenVPNHostEchoServers(t, echoPort)
	serverOptions := option.OpenVPNServerEndpointOptions{
		ListenOptions: option.ListenOptions{
			Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
			ListenPort: openVPNPort,
		},
		Network: N.NetworkUDP,
		Address: []netip.Prefix{netip.MustParsePrefix("10.8.0.1/24")},
		TLS: &option.OpenVPNInboundTLSOptions{
			CertificatePath:       filepath.Join(workspace, "server.crt"),
			KeyPath:               filepath.Join(workspace, "server.key"),
			ClientCertificatePath: filepath.Join(workspace, "ca.crt"),
		},
		Users: []auth.User{
			{
				Username: openVPNTLSUsername,
				Password: openVPNTLSPassword,
			},
		},
	}
	startInstance(t, openVPNServerInstanceOptions(serverOptions))

	writeOpenVPNDockerClientConfig(t, workspace, openVPNPort)
	writeOpenVPNDockerEchoClient(t, workspace, "10.8.0.1", echoPort)
	clientCommand := strings.Join([]string{
		"openvpn --config /config/client.conf &",
		"openvpn_pid=$!",
		"until grep -q 'Initialization Sequence Completed' /config/openvpn.log; do",
		"  if ! kill -0 \"$openvpn_pid\" 2>/dev/null; then cat /config/openvpn.log; exit 1; fi",
		"  sleep 0.1",
		"done",
		"python3 /config/echo_client.py",
		"kill \"$openvpn_pid\"",
		"wait \"$openvpn_pid\" || true",
	}, "\n")
	clientContainer := startOpenVPNDockerContainer(t, dockerClient, "official-client", workspace, clientCommand)
	dumpOpenVPNDockerLogsOnFailure(t, clientContainer, workspace)
	waitResult := clientContainer.Wait(t, 60*time.Second)
	require.Equal(t, int64(0), waitResult.exitCode, waitResult.logs)
}

func waitForOpenVPNClientReady(t *testing.T, proxyPort uint16, echoPort uint16, tunnelAddress string) {
	t.Helper()
	closeEcho := startOpenVPNReadinessEcho(t, echoPort)
	defer closeEcho()
	waitForOpenVPNRemoteReady(t, proxyPort, tunnelAddress, echoPort, 3*time.Minute)
}

func startOpenVPNReadinessEcho(t *testing.T, port uint16) func() {
	t.Helper()
	listener, err := listen("tcp", ":"+strconv.Itoa(int(port)))
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go echoOpenVPNTCPConnection(conn)
		}
	}()
	return func() {
		listener.Close()
		<-done
	}
}

func waitForOpenVPNRemoteReady(t *testing.T, proxyPort uint16, tunnelAddress string, tunnelPort uint16, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = probeOpenVPNTCPWithTimeout(proxyPort, tunnelAddress, tunnelPort, 3*time.Second)
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, lastErr)
}

func probeOpenVPNTCPWithTimeout(proxyPort uint16, tunnelAddress string, tunnelPort uint16, timeout time.Duration) error {
	resultCh := make(chan error, 1)
	go func() {
		probeErr := probeOpenVPNTCP(proxyPort, tunnelAddress, tunnelPort)
		resultCh <- probeErr
	}()
	select {
	case resultErr := <-resultCh:
		return resultErr
	case <-time.After(timeout):
		return E.New("timeout")
	}
}

func probeOpenVPNTCP(proxyPort uint16, tunnelAddress string, tunnelPort uint16) error {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	destination := M.ParseSocksaddrHostPort(tunnelAddress, tunnelPort)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, N.NetworkTCP, destination)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(time.Second))
	if err != nil {
		return err
	}
	return writeAndReadEcho(conn, []byte("ready"))
}

func openVPNServerInstanceOptions(serverOptions option.OpenVPNServerEndpointOptions) option.Options {
	return option.Options{
		Endpoints: []option.Endpoint{
			{
				Type:    C.TypeOpenVPNServer,
				Tag:     "openvpn-server",
				Options: &serverOptions,
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	}
}

func openVPNClientInstanceOptions(clientOptions option.OpenVPNClientEndpointOptions, proxyPort uint16) option.Options {
	return option.Options{
		Endpoints: []option.Endpoint{
			{
				Type:    C.TypeOpenVPNClient,
				Tag:     "openvpn-client",
				Options: &clientOptions,
			},
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeSOCKS,
				Tag:  "socks-in",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
						ListenPort: proxyPort,
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: []string{"socks-in"},
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeRoute,
							RouteOptions: option.RouteActionOptions{
								Outbound: "openvpn-client",
							},
						},
					},
				},
			},
		},
	}
}

func newOpenVPNTLSClientOptions(protocol string, port uint16, certificatePath string, clientCertificatePath string, clientKeyPath string) option.OpenVPNClientEndpointOptions {
	return option.OpenVPNClientEndpointOptions{
		ServerOptions: option.ServerOptions{
			Server:     "127.0.0.1",
			ServerPort: port,
		},
		Network: protocol,
		TLS: &option.OpenVPNOutboundTLSOptions{
			CertificatePath:       certificatePath,
			ClientCertificatePath: clientCertificatePath,
			ClientKeyPath:         clientKeyPath,
		},
	}
}

func writeOpenVPNStaticKeyFile(t *testing.T, staticKey string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "static.key")
	err := os.WriteFile(path, []byte(staticKey), 0o600)
	require.NoError(t, err)
	return path
}

func newOpenVPNDockerWorkspace(t *testing.T, certificates openVPNCertificateBundle) string {
	t.Helper()
	workspace := t.TempDir()
	copyOpenVPNFile(t, certificates.caPath, filepath.Join(workspace, "ca.crt"), 0o600)
	copyOpenVPNFile(t, certificates.serverCertPath, filepath.Join(workspace, "server.crt"), 0o600)
	copyOpenVPNFile(t, certificates.serverKeyPath, filepath.Join(workspace, "server.key"), 0o600)
	copyOpenVPNFile(t, certificates.clientCertPath, filepath.Join(workspace, "client.crt"), 0o600)
	copyOpenVPNFile(t, certificates.clientKeyPath, filepath.Join(workspace, "client.key"), 0o600)
	authFileContent := openVPNTLSUsername + "\n" + openVPNTLSPassword + "\n"
	err := os.WriteFile(filepath.Join(workspace, "auth-user-pass.txt"), []byte(authFileContent), 0o600)
	require.NoError(t, err)
	checkUserPassScript := strings.Join([]string{
		"#!/bin/sh",
		"set -eu",
		"credentials_file=\"$1\"",
		"username=\"$(sed -n '1p' \"$credentials_file\")\"",
		"password=\"$(sed -n '2p' \"$credentials_file\")\"",
		"[ \"$username\" = \"" + openVPNTLSUsername + "\" ] && [ \"$password\" = \"" + openVPNTLSPassword + "\" ]",
		"",
	}, "\n")
	err = os.WriteFile(filepath.Join(workspace, "check_userpass.sh"), []byte(checkUserPassScript), 0o700)
	require.NoError(t, err)
	return workspace
}

func copyOpenVPNFile(t *testing.T, sourcePath string, targetPath string, mode os.FileMode) {
	t.Helper()
	content, err := os.ReadFile(sourcePath)
	require.NoError(t, err)
	err = os.WriteFile(targetPath, content, mode)
	require.NoError(t, err)
}

func openVPNCertificateSHA256Fingerprint(t *testing.T, certificatePath string) string {
	t.Helper()
	certificatePEM, err := os.ReadFile(certificatePath)
	require.NoError(t, err)
	certificateBlock, _ := pem.Decode(certificatePEM)
	require.NotNil(t, certificateBlock)
	require.Equal(t, "CERTIFICATE", certificateBlock.Type)
	fingerprint := sha256.Sum256(certificateBlock.Bytes)
	return hex.EncodeToString(fingerprint[:])
}

func writeOpenVPNDockerServerConfig(t *testing.T, workspace string, openVPNPort uint16, verifyScriptPath string, serverDirectives ...string) {
	t.Helper()
	configLines := []string{
		"port " + strconv.Itoa(int(openVPNPort)),
		"proto udp4",
		"dev tun",
		"topology subnet",
		"server 10.8.0.0 255.255.255.0",
		"ca /config/ca.crt",
		"cert /config/server.crt",
		"key /config/server.key",
		"dh none",
		"persist-key",
		"persist-tun",
		"verb 4",
		"script-security 2",
		"auth-user-pass-verify " + verifyScriptPath + " via-file",
		"explicit-exit-notify 1",
	}
	configLines = append(configLines, serverDirectives...)
	configLines = append(configLines,
		"log /config/openvpn.log",
		"",
	)
	config := strings.Join(configLines, "\n")
	err := os.WriteFile(filepath.Join(workspace, "server.conf"), []byte(config), 0o600)
	require.NoError(t, err)
}

func writeOpenVPNDockerClientConfig(t *testing.T, workspace string, openVPNPort uint16) {
	t.Helper()
	config := strings.Join([]string{
		"client",
		"dev tun",
		"proto udp4",
		"remote 127.0.0.1 " + strconv.Itoa(int(openVPNPort)),
		"resolv-retry infinite",
		"nobind",
		"float",
		"persist-key",
		"persist-tun",
		"remote-cert-tls server",
		"ca /config/ca.crt",
		"cert /config/client.crt",
		"key /config/client.key",
		"auth-user-pass /config/auth-user-pass.txt",
		"verb 4",
		"explicit-exit-notify 1",
		"log /config/openvpn.log",
		"",
	}, "\n")
	err := os.WriteFile(filepath.Join(workspace, "client.conf"), []byte(config), 0o600)
	require.NoError(t, err)
}

func writeOpenVPNDockerEchoServer(t *testing.T, workspace string, host string, port uint16) {
	t.Helper()
	script := strings.ReplaceAll(openVPNDockerEchoServerScript, "{{HOST}}", host)
	script = strings.ReplaceAll(script, "{{PORT}}", strconv.Itoa(int(port)))
	err := os.WriteFile(filepath.Join(workspace, "echo_server.py"), []byte(script), 0o700)
	require.NoError(t, err)
}

func writeOpenVPNDockerEchoClient(t *testing.T, workspace string, host string, port uint16) {
	t.Helper()
	script := strings.ReplaceAll(openVPNDockerEchoClientScript, "{{HOST}}", host)
	script = strings.ReplaceAll(script, "{{PORT}}", strconv.Itoa(int(port)))
	err := os.WriteFile(filepath.Join(workspace, "echo_client.py"), []byte(script), 0o700)
	require.NoError(t, err)
}

func testRemoteEchoThroughSocks(t *testing.T, proxyPort uint16, destinationAddress string, destinationPort uint16) {
	t.Helper()
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	destination := M.ParseSocksaddrHostPort(destinationAddress, destinationPort)
	err := testRemoteTCPEcho(t, dialer, destination)
	require.NoError(t, err)
	err = testRemoteUDPEcho(t, dialer, destination)
	require.NoError(t, err)
}

func testRemoteTCPEcho(t *testing.T, dialer *socks.Client, destination M.Socksaddr) error {
	t.Helper()
	conn, err := dialer.DialContext(context.Background(), N.NetworkTCP, destination)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return err
	}
	err = writeAndReadEcho(conn, []byte("ping"))
	if err != nil {
		return err
	}
	payload := make([]byte, 64*1024)
	for i := 0; i < 100; i++ {
		_, err = rand.Read(payload[1:])
		if err != nil {
			return err
		}
		payload[0] = byte(i)
		err = writeAndReadEcho(conn, payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func testRemoteUDPEcho(t *testing.T, dialer *socks.Client, destination M.Socksaddr) error {
	t.Helper()
	conn, err := dialer.DialContext(context.Background(), N.NetworkUDP, destination)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		return err
	}
	err = writeAndReadPacketEcho(conn, []byte("ping"))
	if err != nil {
		return err
	}
	payload := make([]byte, 1500)
	for i := 0; i < 50; i++ {
		_, err = rand.Read(payload[1:])
		if err != nil {
			return err
		}
		payload[0] = byte(i)
		err = writeAndReadPacketEcho(conn, payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeAndReadEcho(conn net.Conn, payload []byte) error {
	_, err := conn.Write(payload)
	if err != nil {
		return err
	}
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, response) {
		return E.New("unexpected TCP echo response")
	}
	return nil
}

func writeAndReadPacketEcho(conn net.Conn, payload []byte) error {
	_, err := conn.Write(payload)
	if err != nil {
		return err
	}
	response := make([]byte, len(payload)+512)
	n, err := conn.Read(response)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, response[:n]) {
		return E.New("unexpected UDP echo response")
	}
	return nil
}

func startOpenVPNHostEchoServers(t *testing.T, port uint16) {
	t.Helper()
	tcpListener, err := listen("tcp", ":"+strconv.Itoa(int(port)))
	require.NoError(t, err)
	udpConnection, err := listenPacket("udp", ":"+strconv.Itoa(int(port)))
	require.NoError(t, err)
	t.Cleanup(func() {
		tcpListener.Close()
		udpConnection.Close()
	})
	go func() {
		for {
			conn, acceptErr := tcpListener.Accept()
			if acceptErr != nil {
				return
			}
			go echoOpenVPNTCPConnection(conn)
		}
	}()
	go func() {
		buffer := make([]byte, 64*1024)
		for {
			n, address, readErr := udpConnection.ReadFrom(buffer)
			if readErr != nil {
				return
			}
			_, _ = udpConnection.WriteTo(buffer[:n], address)
		}
	}()
}

func echoOpenVPNTCPConnection(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 64*1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		_, err = conn.Write(buffer[:n])
		if err != nil {
			return
		}
	}
}

type openVPNDockerContainer struct {
	dockerClient *client.Client
	containerID  string
	name         string
}

type openVPNDockerWaitResult struct {
	exitCode int64
	logs     string
}

func requireOpenVPNDockerEnvironment(t *testing.T) *client.Client {
	t.Helper()
	dockerClient := openVPNDockerClientForTest(t)
	_, err := dockerClient.Ping(context.Background())
	if err != nil {
		dockerClient.Close()
		t.Skipf("Docker is unavailable: %v", err)
	}
	openVPNDockerImageOnce.Do(func() {
		openVPNDockerImageErr = ensureOpenVPNDockerImage(dockerClient)
	})
	require.NoError(t, openVPNDockerImageErr)
	verifyOpenVPNDockerImage(t, dockerClient)
	return dockerClient
}

func openVPNDockerClientForTest(t *testing.T) *client.Client {
	t.Helper()
	clientOptions := []client.Opt{client.WithAPIVersionNegotiation()}
	dockerHost := os.Getenv("DOCKER_HOST")
	switch {
	case dockerHost != "":
		clientOptions = append(clientOptions, client.WithHost(dockerHost))
	case openVPNFileExists("/Users/sekai/.orbstack/run/docker.sock"):
		clientOptions = append(clientOptions, client.WithHost("unix:///Users/sekai/.orbstack/run/docker.sock"))
	case openVPNFileExists("/var/run/docker.sock"):
		clientOptions = append(clientOptions, client.WithHost("unix:///var/run/docker.sock"))
	default:
		t.Skip("Docker is unavailable: docker socket not found")
	}
	dockerClient, err := client.NewClientWithOpts(clientOptions...)
	require.NoError(t, err)
	t.Cleanup(func() {
		dockerClient.Close()
	})
	return dockerClient
}

func openVPNFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ensureOpenVPNDockerImage(dockerClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _, err := dockerClient.ImageInspectWithRaw(ctx, openVPNDockerImage)
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return err
	}
	return buildOpenVPNDockerImage(dockerClient)
}

func buildOpenVPNDockerImage(dockerClient *client.Client) error {
	dockerfile := strings.Join([]string{
		"FROM debian:bookworm-slim",
		"ARG OPENVPN_VERSION=" + openVPNDockerPackageVersion,
		"RUN apt-get update \\",
		"    && apt-get install -y --no-install-recommends \\",
		"        bash \\",
		"        ca-certificates \\",
		"        grep \\",
		"        iproute2 \\",
		"        iputils-ping \\",
		"        openvpn=${OPENVPN_VERSION} \\",
		"        procps \\",
		"        python3 \\",
		"        tcpdump \\",
		"    && rm -rf /var/lib/apt/lists/*",
		"RUN openvpn --version | head -n 1 | grep 'OpenVPN 2.6.14'",
		"",
	}, "\n")
	var buffer bytes.Buffer
	tarWriter := tar.NewWriter(&buffer)
	header := &tar.Header{
		Name: "Dockerfile",
		Mode: 0o644,
		Size: int64(len(dockerfile)),
	}
	err := tarWriter.WriteHeader(header)
	if err == nil {
		_, err = tarWriter.Write([]byte(dockerfile))
	}
	closeErr := tarWriter.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	buildOptions := typesapi.ImageBuildOptions{
		Tags:       []string{openVPNDockerImage},
		Remove:     true,
		PullParent: true,
		Platform:   openVPNDockerPlatform(),
	}
	response, err := dockerClient.ImageBuild(ctx, bytes.NewReader(buffer.Bytes()), buildOptions)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var output bytes.Buffer
	err = jsonmessage.DisplayJSONMessagesStream(response.Body, &output, 0, false, nil)
	if err != nil {
		return E.Cause(err, "build OpenVPN docker image\n", output.String())
	}
	return nil
}

func verifyOpenVPNDockerImage(t *testing.T, dockerClient *client.Client) {
	t.Helper()
	containerConfig := &containerapi.Config{
		Image: openVPNDockerImage,
		Cmd:   []string{"openvpn", "--version"},
	}
	createdContainer, err := dockerClient.ContainerCreate(context.Background(), containerConfig, &containerapi.HostConfig{}, nil, openVPNDockerOCIPlatform(), "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = removeOpenVPNDockerContainer(context.Background(), dockerClient, createdContainer.ID)
	})
	err = dockerClient.ContainerStart(context.Background(), createdContainer.ID, containerapi.StartOptions{})
	require.NoError(t, err)
	containerHandle := &openVPNDockerContainer{
		dockerClient: dockerClient,
		containerID:  createdContainer.ID,
		name:         "openvpn-version",
	}
	waitResult := containerHandle.Wait(t, 30*time.Second)
	require.Equal(t, int64(0), waitResult.exitCode, waitResult.logs)
	firstLine, _, _ := strings.Cut(waitResult.logs, "\n")
	require.Contains(t, firstLine, "OpenVPN 2.6.14")
}

func startOpenVPNDockerContainer(t *testing.T, dockerClient *client.Client, nameSuffix string, workspace string, command string) *openVPNDockerContainer {
	t.Helper()
	name := "sing-box-openvpn-" + nameSuffix + "-" + sanitizeOpenVPNDockerName(t.Name())
	containerConfig := &containerapi.Config{
		Image: openVPNDockerImage,
		Cmd:   []string{"bash", "-lc", command},
	}
	hostConfig := &containerapi.HostConfig{
		NetworkMode: containerapi.NetworkMode("host"),
		Binds:       []string{workspace + ":" + openVPNDockerRoot},
		CapAdd:      []string{"NET_ADMIN"},
		Resources: containerapi.Resources{
			Devices: []containerapi.DeviceMapping{
				{
					PathOnHost:        "/dev/net/tun",
					PathInContainer:   "/dev/net/tun",
					CgroupPermissions: "rwm",
				},
			},
		},
	}
	createdContainer, err := dockerClient.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, openVPNDockerOCIPlatform(), name)
	require.NoError(t, err)
	containerHandle := &openVPNDockerContainer{
		dockerClient: dockerClient,
		containerID:  createdContainer.ID,
		name:         name,
	}
	t.Cleanup(func() {
		_ = removeOpenVPNDockerContainer(context.Background(), dockerClient, createdContainer.ID)
	})
	err = dockerClient.ContainerStart(context.Background(), createdContainer.ID, containerapi.StartOptions{})
	require.NoError(t, err)
	return containerHandle
}

func waitForOpenVPNDockerFile(t *testing.T, containerHandle *openVPNDockerContainer, path string, content string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		fileContent, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(fileContent), content) {
			return
		}
		containerHandle.failIfExited(t)
		time.Sleep(100 * time.Millisecond)
	}
	fileContent, _ := os.ReadFile(path)
	t.Fatalf("timed out waiting for %q in %s\nfile:\n%s\ncontainer logs:\n%s", content, path, string(fileContent), containerHandle.Logs(context.Background()))
}

func dumpOpenVPNDockerLogsOnFailure(t *testing.T, containerHandle *openVPNDockerContainer, workspace string) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("container logs for %s:\n%s", containerHandle.name, containerHandle.Logs(context.Background()))
		openVPNLog, err := os.ReadFile(filepath.Join(workspace, "openvpn.log"))
		if err == nil {
			t.Logf("openvpn log for %s:\n%s", containerHandle.name, string(openVPNLog))
		}
	})
}

func (c *openVPNDockerContainer) failIfExited(t *testing.T) {
	t.Helper()
	containerInfo, err := c.dockerClient.ContainerInspect(context.Background(), c.containerID)
	require.NoError(t, err)
	if containerInfo.State != nil && !containerInfo.State.Running {
		t.Fatalf("docker container %s exited with code %d\nlogs:\n%s", c.name, containerInfo.State.ExitCode, c.Logs(context.Background()))
	}
}

func (c *openVPNDockerContainer) Wait(t *testing.T, timeout time.Duration) openVPNDockerWaitResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	statusChannel, errChannel := c.dockerClient.ContainerWait(ctx, c.containerID, containerapi.WaitConditionNotRunning)
	select {
	case waitErr := <-errChannel:
		if waitErr != nil {
			t.Fatalf("wait docker container %s: %v\nlogs:\n%s", c.name, waitErr, c.Logs(context.Background()))
		}
	case status := <-statusChannel:
		return openVPNDockerWaitResult{
			exitCode: status.StatusCode,
			logs:     c.Logs(context.Background()),
		}
	case <-ctx.Done():
		t.Fatalf("docker container %s timed out\nlogs:\n%s", c.name, c.Logs(context.Background()))
	}
	return openVPNDockerWaitResult{}
}

func (c *openVPNDockerContainer) Logs(ctx context.Context) string {
	logReader, err := c.dockerClient.ContainerLogs(ctx, c.containerID, containerapi.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "read logs: " + err.Error()
	}
	defer logReader.Close()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, logReader)
	if err != nil {
		return "decode logs: " + err.Error()
	}
	if stderr.Len() == 0 {
		return stdout.String()
	}
	if stdout.Len() == 0 {
		return stderr.String()
	}
	return stdout.String() + "\nSTDERR:\n" + stderr.String()
}

func removeOpenVPNDockerContainer(ctx context.Context, dockerClient *client.Client, containerID string) error {
	err := dockerClient.ContainerRemove(ctx, containerID, containerapi.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func openVPNDockerPlatform() string {
	switch runtime.GOARCH {
	case "arm64":
		return "linux/arm64"
	case "amd64":
		return "linux/amd64"
	default:
		return ""
	}
}

func openVPNDockerOCIPlatform() *ocispec.Platform {
	switch runtime.GOARCH {
	case "arm64", "amd64":
		return &ocispec.Platform{
			OS:           "linux",
			Architecture: runtime.GOARCH,
		}
	default:
		return nil
	}
}

func sanitizeOpenVPNDockerName(name string) string {
	replacer := strings.NewReplacer("/", "-", "_", "-", " ", "-")
	return replacer.Replace(name)
}

const openVPNDockerEchoServerScript = `#!/usr/bin/env python3
import os
import socket
import threading
import time

HOST = "{{HOST}}"
PORT = {{PORT}}
READY = "/config/echo.ready"


def bind_with_retry(sock, address):
    last_error = None
    for _ in range(300):
        try:
            sock.bind(address)
            return
        except OSError as error:
            last_error = error
            time.sleep(0.1)
    raise last_error


def handle_tcp(conn):
    with conn:
        while True:
            data = conn.recv(65536)
            if not data:
                return
            conn.sendall(data)


def tcp_server():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    bind_with_retry(sock, (HOST, PORT))
    sock.listen(64)
    print("tcp echo ready", flush=True)
    while True:
        conn, _ = sock.accept()
        threading.Thread(target=handle_tcp, args=(conn,), daemon=True).start()


def udp_server():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    bind_with_retry(sock, (HOST, PORT))
    print("udp echo ready", flush=True)
    while True:
        data, address = sock.recvfrom(65536)
        sock.sendto(data, address)


threading.Thread(target=tcp_server, daemon=True).start()
threading.Thread(target=udp_server, daemon=True).start()
time.sleep(0.2)
with open(READY, "w", encoding="utf-8") as ready_file:
    ready_file.write("ready\n")
while True:
    time.sleep(3600)
`

const openVPNDockerEchoClientScript = `#!/usr/bin/env python3
import os
import socket

HOST = "{{HOST}}"
PORT = {{PORT}}


def check_tcp():
    sock = socket.create_connection((HOST, PORT), timeout=10)
    sock.settimeout(10)
    with sock:
        payload = b"ping"
        sock.sendall(payload)
        response = sock.recv(len(payload))
        if response != payload:
            raise RuntimeError("unexpected tcp ping response")
        for index in range(100):
            payload = bytes([index]) + os.urandom(64 * 1024 - 1)
            sock.sendall(payload)
            response = bytearray()
            while len(response) < len(payload):
                chunk = sock.recv(len(payload) - len(response))
                if not chunk:
                    raise RuntimeError("tcp echo closed")
                response.extend(chunk)
            if bytes(response) != payload:
                raise RuntimeError("unexpected tcp large response")


def check_udp():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.settimeout(10)
    sock.connect((HOST, PORT))
    with sock:
        payload = b"ping"
        sock.send(payload)
        response = sock.recv(65536)
        if response != payload:
            raise RuntimeError("unexpected udp ping response")
        for index in range(50):
            payload = bytes([index]) + os.urandom(1499)
            sock.send(payload)
            response = sock.recv(65536)
            if response != payload:
                raise RuntimeError("unexpected udp large response")


check_tcp()
check_udp()
print("echo client ok", flush=True)
`

func testSuitOpenVPN(t *testing.T, proxyPort uint16, echoPort uint16, tunnelAddress string) {
	t.Helper()
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	destination := M.ParseSocksaddrHostPort(tunnelAddress, echoPort)
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), N.NetworkTCP, destination)
	}
	dialUDP := func() (net.PacketConn, error) {
		conn, err := dialer.DialContext(context.Background(), N.NetworkUDP, destination)
		if err != nil {
			return nil, err
		}
		return bufio.NewUnbindPacketConn(conn), nil
	}
	require.NoError(t, testOpenVPNEchoWithConn(echoPort, dialTCP))
	require.NoError(t, testOpenVPNEchoWithPacketConn(echoPort, dialUDP))
	require.NoError(t, testOpenVPNLargeDataWithConn(echoPort, dialTCP))
}

func testOpenVPNEchoWithConn(port uint16, dialTCP func() (net.Conn, error)) error {
	listener, err := listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	serverErrCh := make(chan error, 1)
	go func() {
		serverConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer serverConn.Close()
		deadlineErr := serverConn.SetDeadline(time.Now().Add(openVPNLargeDataTimeout))
		if deadlineErr != nil {
			serverErrCh <- deadlineErr
			return
		}
		buffer := make([]byte, 4)
		_, readErr := io.ReadFull(serverConn, buffer)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		_, writeErr := serverConn.Write(buffer)
		if writeErr != nil {
			serverErrCh <- writeErr
			return
		}
		serverErrCh <- nil
	}()
	conn, err := dialTCP()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(openVPNLargeDataTimeout))
	if err != nil {
		return err
	}
	err = writeAndReadEcho(conn, []byte("ping"))
	if err != nil {
		return err
	}
	return <-serverErrCh
}

func testOpenVPNEchoWithPacketConn(port uint16, listenUDP func() (net.PacketConn, error)) error {
	listener, err := listenPacket("udp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	serverErrCh := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1024)
		readCount, address, readErr := listener.ReadFrom(buffer)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		_, writeErr := listener.WriteTo(buffer[:readCount], address)
		if writeErr != nil {
			serverErrCh <- writeErr
			return
		}
		serverErrCh <- nil
	}()
	packetConn, err := listenUDP()
	if err != nil {
		return err
	}
	defer packetConn.Close()
	remoteAddress := &net.UDPAddr{IP: localIP.AsSlice(), Port: int(port)}
	payload := []byte("ping")
	_, err = packetConn.WriteTo(payload, remoteAddress)
	if err != nil {
		return err
	}
	response := make([]byte, 1024)
	readCount, err := readOpenVPNPacketWithTimeout(packetConn, response)
	if err != nil {
		return err
	}
	if !bytes.Equal(response[:readCount], payload) {
		return E.New("unexpected UDP echo response")
	}
	return <-serverErrCh
}

func testOpenVPNLargeDataWithConn(port uint16, dialTCP func() (net.Conn, error)) error {
	listener, err := listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	serverErrCh := make(chan error, 1)
	go func() {
		serverConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer serverConn.Close()
		deadlineErr := serverConn.SetDeadline(time.Now().Add(openVPNLargeDataTimeout))
		if deadlineErr != nil {
			serverErrCh <- deadlineErr
			return
		}
		buffer := make([]byte, openVPNLargeTCPSize)
		for range openVPNLargeTCPPackets {
			_, readErr := io.ReadFull(serverConn, buffer)
			if readErr != nil {
				serverErrCh <- readErr
				return
			}
			_, writeErr := serverConn.Write(buffer)
			if writeErr != nil {
				serverErrCh <- writeErr
				return
			}
		}
		serverErrCh <- nil
	}()
	conn, err := dialTCP()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(openVPNLargeDataTimeout))
	if err != nil {
		return err
	}
	response := make([]byte, openVPNLargeTCPSize)
	for index := range openVPNLargeTCPPackets {
		payload := make([]byte, openVPNLargeTCPSize)
		payload[0] = byte(index)
		_, err = rand.Read(payload[1:])
		if err != nil {
			return err
		}
		_, err = conn.Write(payload)
		if err != nil {
			return err
		}
		_, err = io.ReadFull(conn, response)
		if err != nil {
			return err
		}
		if !bytes.Equal(response, payload) {
			return E.New("unexpected tcp large response")
		}
	}
	return <-serverErrCh
}

type openVPNPacketReadResult struct {
	readCount int
	err       error
}

func readOpenVPNPacketWithTimeout(packetConn net.PacketConn, buffer []byte) (int, error) {
	resultCh := make(chan openVPNPacketReadResult, 1)
	go func() {
		readCount, _, readErr := packetConn.ReadFrom(buffer)
		resultCh <- openVPNPacketReadResult{
			readCount: readCount,
			err:       readErr,
		}
	}()
	select {
	case result := <-resultCh:
		return result.readCount, result.err
	case <-time.After(openVPNLargeDataTimeout):
		return 0, E.New("timeout")
	}
}

func createOpenVPNCertificateBundle(t *testing.T) openVPNCertificateBundle {
	t.Helper()
	tempDir := t.TempDir()
	caKey, err := rsa.GenerateKey(rand.Reader, 3072)
	require.NoError(t, err)
	spkiASN1, err := x509.MarshalPKIXPublicKey(caKey.Public())
	require.NoError(t, err)
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	_, err = asn1.Unmarshal(spkiASN1, &spki)
	require.NoError(t, err)
	subjectKeyID := sha1.Sum(spki.SubjectPublicKey.Bytes)
	caTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject: pkix.Name{
			Organization: []string{"sing-box OpenVPN test CA"},
			CommonName:   "sing-box OpenVPN test CA",
		},
		SubjectKeyId:          subjectKeyID[:],
		NotAfter:              time.Now().AddDate(1, 0, 0),
		NotBefore:             time.Now().Add(-time.Minute),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caCertificate, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caKey.Public(), caKey)
	require.NoError(t, err)
	caPath := filepath.Join(tempDir, "ca.crt")
	writePEMFile(t, caPath, "CERTIFICATE", caCertificate)
	serverCertPath, serverKeyPath := createOpenVPNLeafCertificate(t, tempDir, "server", x509.ExtKeyUsageServerAuth, caTemplate, caKey)
	clientCertPath, clientKeyPath := createOpenVPNLeafCertificate(t, tempDir, "client", x509.ExtKeyUsageClientAuth, caTemplate, caKey)
	return openVPNCertificateBundle{
		caPath:         caPath,
		serverCertPath: serverCertPath,
		serverKeyPath:  serverKeyPath,
		clientCertPath: clientCertPath,
		clientKeyPath:  clientKeyPath,
	}
}

func createOpenVPNLeafCertificate(t *testing.T, tempDir string, commonName string, usage x509.ExtKeyUsage, caTemplate *x509.Certificate, caKey *rsa.PrivateKey) (string, string) {
	t.Helper()
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	leafTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(t),
		Subject: pkix.Name{
			Organization: []string{"sing-box OpenVPN test"},
			CommonName:   commonName,
		},
		NotBefore: time.Now().Add(-time.Minute),
		NotAfter:  time.Now().AddDate(0, 1, 0),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			usage,
		},
	}
	if usage == x509.ExtKeyUsageServerAuth {
		leafTemplate.IPAddresses = append(leafTemplate.IPAddresses, net.ParseIP("127.0.0.1"))
		leafTemplate.DNSNames = append(leafTemplate.DNSNames, "localhost")
	}
	leafCertificate, err := x509.CreateCertificate(rand.Reader, leafTemplate, caTemplate, leafKey.Public(), caKey)
	require.NoError(t, err)
	certPath := filepath.Join(tempDir, commonName+".crt")
	keyPath := filepath.Join(tempDir, commonName+".key")
	writePEMFile(t, certPath, "CERTIFICATE", leafCertificate)
	privateKey, err := x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)
	writePEMFile(t, keyPath, "PRIVATE KEY", privateKey)
	return certPath, keyPath
}

func writePEMFile(t *testing.T, path string, blockType string, bytes []byte) {
	t.Helper()
	content := pem.EncodeToMemory(&pem.Block{
		Type:  blockType,
		Bytes: bytes,
	})
	err := os.WriteFile(path, content, 0o600)
	require.NoError(t, err)
}

func createOpenVPNStaticKey(t *testing.T) string {
	t.Helper()
	keyMaterial := make([]byte, 256)
	_, err := rand.Read(keyMaterial)
	require.NoError(t, err)
	hexKey := hex.EncodeToString(keyMaterial)
	lines := []string{"-----BEGIN OpenVPN Static key V1-----"}
	for index := 0; index < len(hexKey); index += 32 {
		lines = append(lines, hexKey[index:index+32])
	}
	lines = append(lines, "-----END OpenVPN Static key V1-----", "")
	return strings.Join(lines, "\n")
}

func reserveOpenVPNProtocolPort(t *testing.T, protocol string) uint16 {
	t.Helper()
	if protocol == N.NetworkTCP {
		return reserveOpenVPNTCPPort(t)
	}
	return reserveOpenVPNUDPPort(t)
}

func reserveOpenVPNTCPPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	tcpAddress := listener.Addr().(*net.TCPAddr)
	return uint16(tcpAddress.Port)
}

func reserveOpenVPNUDPPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	udpAddress := listener.LocalAddr().(*net.UDPAddr)
	return uint16(udpAddress.Port)
}

func reserveOpenVPNEchoPort(t *testing.T) uint16 {
	t.Helper()
	for i := 0; i < 20; i++ {
		tcpListener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		tcpAddress := tcpListener.Addr().(*net.TCPAddr)
		port := uint16(tcpAddress.Port)
		udpListener, err := net.ListenPacket("udp", ":"+strconv.Itoa(int(port)))
		if err == nil {
			udpListener.Close()
			tcpListener.Close()
			return port
		}
		tcpListener.Close()
	}
	t.Fatal("reserve TCP and UDP echo port")
	return 0
}
