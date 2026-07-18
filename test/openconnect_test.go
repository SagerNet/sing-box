package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

const (
	openConnectInteropEnvironment = "OPENCONNECT_IT"
	openConnectOcservVersion      = "1.3.0-2"
	openConnectOcservImage        = "sing-box-openconnect-ocserv:" + openConnectOcservVersion
	openConnectUsername           = "test"
	openConnectPassword           = "test"
	openConnectTunnelAddress      = "192.168.77.1"
	openConnectEchoPort           = 18080
)

const openConnectOcservPasswordFile = "test:tost,group1,group2:$5$i6SNmLDCgBNjyJ7q$SZ4bVJb7I/DLgXo3txHBVohRFBjOtdbxGQZp.DOnrA.\n"

const openConnectOcservConfiguration = `auth = "plain[passwd=/fixture/ocpasswd]"

tcp-port = 443
udp-port = 443

run-as-user = nobody
run-as-group = nogroup
socket-file = /run/ocserv-socket
use-occtl = true
occtl-socket-file = /run/occtl.socket

server-cert = /fixture/server-cert.pem
server-key = /fixture/server-key.pem
tls-priorities = "NORMAL:%SERVER_PRECEDENCE:%COMPAT"

isolate-workers = false
max-clients = 4
max-same-clients = 2
rate-limit-ms = 0
max-ban-score = 0
auth-timeout = 30
cookie-timeout = 300
keepalive = 1
dpd = 2
try-mtu-discovery = false

device = vpns
ipv4-network = 192.168.77.0
ipv4-netmask = 255.255.255.0
route = 192.168.77.0/255.255.255.0
ping-leases = false
mtu = 1400

cisco-client-compat = false
dtls-psk = true
dtls-legacy = false
match-tls-dtls-ciphers = false
rekey-time = 0
rekey-method = new-tunnel
`

type openConnectOcservContainer struct {
	name                     string
	tcpAddress               string
	serverAddress            string
	certificateAuthorityPath string
	passwordPath             string
}

type openConnectTCPProxy struct {
	listener    net.Listener
	target      string
	access      sync.Mutex
	connections map[*openConnectTCPProxyConnection]struct{}
	closed      bool
	accepted    atomic.Uint64
}

type openConnectTCPProxyConnection struct {
	proxy      *openConnectTCPProxy
	downstream net.Conn
	upstream   net.Conn
	closeOnce  sync.Once
}

func TestOpenConnectDockerInterop(t *testing.T) {
	if testing.Short() || strings.TrimSpace(os.Getenv(openConnectInteropEnvironment)) == "" {
		t.Skip(openConnectInteropEnvironment + " is not set or short testing is enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)
	requireOpenConnectDockerImage(t, ctx)

	t.Run("prefilled_credentials_and_tcp_echo", func(subtest *testing.T) {
		container := startOpenConnectOcservContainer(subtest, ctx)
		instance := startInstance(subtest, openConnectInstanceOptions(
			container.serverAddress,
			container.certificateAuthorityPath,
			openConnectUsername,
			openConnectPassword,
		))
		endpoint := requireOpenConnectEndpoint(subtest, instance)
		status := waitForOpenConnectState(subtest, endpoint, adapter.OpenConnectStateConnected, 45*time.Second)
		require.Nil(subtest, status.AuthForm)
		err := exchangeOpenConnectTCPEcho(endpoint, 256*1024, 30*time.Second)
		require.NoError(subtest, err)
		err = exchangeOpenConnectUDPEcho(endpoint, 1400, 30*time.Second)
		require.NoError(subtest, err)
	})

	t.Run("interactive_password_auth", func(subtest *testing.T) {
		container := startOpenConnectOcservContainer(subtest, ctx)
		instance := startInstance(subtest, openConnectInstanceOptions(
			container.serverAddress,
			container.certificateAuthorityPath,
			"",
			"",
		))
		endpoint := requireOpenConnectEndpoint(subtest, instance)
		driveOpenConnectInteractiveAuthentication(subtest, endpoint, 45*time.Second)
		err := exchangeOpenConnectTCPEcho(endpoint, 64*1024, 30*time.Second)
		require.NoError(subtest, err)
	})

	t.Run("cstp_reconnect_reuses_cookie", func(subtest *testing.T) {
		container := startOpenConnectOcservContainer(subtest, ctx)
		proxy := startOpenConnectTCPProxy(subtest, container.tcpAddress)
		instance := startInstance(subtest, openConnectInstanceOptions(
			openConnectLocalhostAddress(subtest, proxy.listener.Addr().String()),
			container.certificateAuthorityPath,
			openConnectUsername,
			openConnectPassword,
		))
		endpoint := requireOpenConnectEndpoint(subtest, instance)
		waitForOpenConnectState(subtest, endpoint, adapter.OpenConnectStateConnected, 45*time.Second)
		waitForOpenConnectTCPEcho(subtest, endpoint, 30*time.Second)

		acceptedBeforeDrop := proxy.accepted.Load()
		err := os.WriteFile(container.passwordPath, []byte("test:tost,group1,group2:!\n"), 0o644)
		require.NoError(subtest, err)
		droppedConnections := proxy.dropConnections()
		require.Positive(subtest, droppedConnections)
		waitForOpenConnectProxyAccept(subtest, proxy, acceptedBeforeDrop, 30*time.Second)
		waitForOpenConnectTCPEcho(subtest, endpoint, 60*time.Second)

		status := endpoint.OpenConnectStatus()
		require.Equal(subtest, adapter.OpenConnectStateConnected, status.State, status.Error)
		require.Nil(subtest, status.AuthForm)
		logs, err := openConnectDockerOutput(ctx, "logs", container.name)
		require.NoError(subtest, err)
		require.GreaterOrEqual(subtest, strings.Count(logs, "HTTP CONNECT /CSCOSSLC/tunnel"), 2, logs)
		require.Equal(subtest, 1, strings.Count(logs, "user '"+openConnectUsername+"' obtained cookie"), logs)
	})
}

func openConnectInstanceOptions(server string, certificateAuthorityPath string, username string, password string) option.Options {
	endpointOptions := option.OpenConnectEndpointOptions{
		Server:       server,
		Flavor:       "anyconnect",
		Username:     username,
		Password:     password,
		NoUDP:        true,
		UDPTimeout:   badoption.Duration(time.Minute),
		UDPMapping:   option.UDPNATBehaviorAddressDependent,
		UDPFiltering: option.UDPNATBehaviorAddressAndPortDependent,
		UDPNATMax:    128,
		TLS: option.OpenConnectTLSOptions{
			CertificateAuthorityPath: certificateAuthorityPath,
		},
	}
	return option.Options{
		Endpoints: []option.Endpoint{
			{
				Type:    C.TypeOpenConnect,
				Tag:     "openconnect-client",
				Options: &endpointOptions,
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: C.TypeDirect,
			},
		},
	}
}

func requireOpenConnectEndpoint(t *testing.T, instance *box.Box) adapter.OpenConnectEndpoint {
	t.Helper()
	endpoint, loaded := instance.Endpoint().Get("openconnect-client")
	require.True(t, loaded)
	openConnectEndpoint, supported := endpoint.(adapter.OpenConnectEndpoint)
	require.True(t, supported)
	return openConnectEndpoint
}

func waitForOpenConnectState(t *testing.T, endpoint adapter.OpenConnectEndpoint, expectedState string, timeout time.Duration) adapter.OpenConnectStatus {
	t.Helper()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	for {
		statusUpdated := endpoint.StatusUpdated()
		status := endpoint.OpenConnectStatus()
		if status.State == expectedState {
			return status
		}
		if status.State == adapter.OpenConnectStateError {
			t.Fatalf("OpenConnect endpoint failed while waiting for %q: %s", expectedState, status.Error)
		}
		select {
		case <-statusUpdated:
		case <-timeoutTimer.C:
			t.Fatalf("timed out waiting for OpenConnect state %q; last state %q, error %q", expectedState, status.State, status.Error)
		}
	}
}

func driveOpenConnectInteractiveAuthentication(t *testing.T, endpoint adapter.OpenConnectEndpoint, timeout time.Duration) {
	t.Helper()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	completedForms := make(map[string]struct{})
	sawUsername := false
	sawPassword := false
	for {
		statusUpdated := endpoint.StatusUpdated()
		status := endpoint.OpenConnectStatus()
		switch status.State {
		case adapter.OpenConnectStateConnected:
			require.True(t, sawUsername)
			require.True(t, sawPassword)
			return
		case adapter.OpenConnectStateError:
			t.Fatal(status.Error)
		case adapter.OpenConnectStateAuthPending:
			form := status.AuthForm
			require.NotNil(t, form)
			require.NotEmpty(t, form.ID)
			_, completed := completedForms[form.ID]
			if !completed {
				values := make(map[string]string, len(form.Fields))
				for _, field := range form.Fields {
					require.NotEmpty(t, field.SubmissionKey)
					switch field.Name {
					case "username":
						sawUsername = true
						values[field.SubmissionKey] = openConnectUsername
					case "password":
						sawPassword = true
						values[field.SubmissionKey] = openConnectPassword
					default:
						t.Fatalf("unexpected ocserv authentication field: %#v", field)
					}
				}
				require.NotEmpty(t, values)
				err := endpoint.CompleteAuthForm(form.ID, values)
				require.NoError(t, err)
				completedForms[form.ID] = struct{}{}
				continue
			}
		}
		select {
		case <-statusUpdated:
		case <-timeoutTimer.C:
			t.Fatalf("timed out driving OpenConnect authentication; last state %q, error %q", status.State, status.Error)
		}
	}
}

func exchangeOpenConnectTCPEcho(endpoint adapter.OpenConnectEndpoint, payloadSize int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := endpoint.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddrHostPort(openConnectTunnelAddress, openConnectEchoPort))
	if err != nil {
		return E.Cause(err, "dial ocserv tunnel echo")
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return E.Cause(err, "set ocserv tunnel echo deadline")
	}
	payload := make([]byte, payloadSize)
	_, err = rand.Read(payload)
	if err != nil {
		return E.Cause(err, "generate ocserv tunnel echo payload")
	}
	written := 0
	for written < len(payload) {
		var n int
		n, err = conn.Write(payload[written:])
		if err != nil {
			return E.Cause(err, "write ocserv tunnel echo payload")
		}
		written += n
	}
	response := make([]byte, len(payload))
	_, err = io.ReadFull(conn, response)
	if err != nil {
		return E.Cause(err, "read ocserv tunnel echo payload")
	}
	if !bytes.Equal(response, payload) {
		return E.New("ocserv tunnel echo payload mismatch")
	}
	return nil
}

func exchangeOpenConnectUDPEcho(endpoint adapter.OpenConnectEndpoint, payloadSize int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := endpoint.DialContext(ctx, N.NetworkUDP, M.ParseSocksaddrHostPort(openConnectTunnelAddress, openConnectEchoPort))
	if err != nil {
		return E.Cause(err, "dial ocserv tunnel UDP echo")
	}
	defer conn.Close()
	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return E.Cause(err, "set ocserv tunnel UDP echo deadline")
	}
	payload := make([]byte, payloadSize)
	_, err = rand.Read(payload)
	if err != nil {
		return E.Cause(err, "generate ocserv tunnel UDP echo payload")
	}
	_, err = conn.Write(payload)
	if err != nil {
		return E.Cause(err, "write ocserv tunnel UDP echo payload")
	}
	response := make([]byte, payloadSize+1)
	responseLength, err := conn.Read(response)
	if err != nil {
		return E.Cause(err, "read ocserv tunnel UDP echo payload")
	}
	if !bytes.Equal(response[:responseLength], payload) {
		return E.New("ocserv tunnel UDP echo payload mismatch")
	}
	return nil
}

func waitForOpenConnectTCPEcho(t *testing.T, endpoint adapter.OpenConnectEndpoint, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = exchangeOpenConnectTCPEcho(endpoint, 4096, 3*time.Second)
		if lastErr == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr == nil {
		t.Fatal("timed out before attempting OpenConnect tunnel echo")
	}
	t.Fatal(E.Cause(lastErr, "timed out waiting for OpenConnect tunnel echo"))
}

func requireOpenConnectDockerImage(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := openConnectDockerOutput(ctx, "version", "--format", "{{.Server.Version}}")
	require.NoError(t, err)
	buildContext, err := filepath.Abs(filepath.Join("testdata", "openconnect", "ocserv"))
	require.NoError(t, err)
	_, err = openConnectDockerOutput(ctx, "build", "--pull=false", "--tag", openConnectOcservImage, buildContext)
	require.NoError(t, err)
}

func startOpenConnectOcservContainer(t *testing.T, ctx context.Context) openConnectOcservContainer {
	t.Helper()
	certificateAuthorityPath, certificatePath, keyPath := createSelfSignedCertificate(t, "localhost")
	workspace := t.TempDir()
	err := os.Chmod(workspace, 0o755)
	require.NoError(t, err)
	certificate, err := os.ReadFile(certificatePath)
	require.NoError(t, err)
	key, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	serverCertificatePath := filepath.Join(workspace, "server-cert.pem")
	serverKeyPath := filepath.Join(workspace, "server-key.pem")
	passwordPath := filepath.Join(workspace, "ocpasswd")
	err = os.WriteFile(serverCertificatePath, certificate, 0o644)
	require.NoError(t, err)
	err = os.WriteFile(serverKeyPath, key, 0o600)
	require.NoError(t, err)
	err = os.WriteFile(passwordPath, []byte(openConnectOcservPasswordFile), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(workspace, "ocserv.conf"), []byte(openConnectOcservConfiguration), 0o644)
	require.NoError(t, err)

	containerName := "sing-box-openconnect-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	_, err = openConnectDockerOutput(
		ctx,
		"run", "--detach", "--rm", "--name", containerName,
		"--cap-add", "NET_ADMIN", "--device", "/dev/net/tun",
		"--publish", "127.0.0.1::443/tcp",
		"--mount", "type=bind,source="+workspace+",target=/fixture",
		"--entrypoint", "sh",
		openConnectOcservImage,
		"-c", "python3 /usr/local/bin/openconnect-echo-server & exec ocserv -f -d 4 -c /fixture/ocserv.conf",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if t.Failed() {
			logsContext, cancelLogs := context.WithTimeout(context.Background(), 5*time.Second)
			logs, logsErr := openConnectDockerOutput(logsContext, "logs", containerName)
			cancelLogs()
			if logsErr == nil {
				t.Log("ocserv logs:\n" + logs)
			}
		}
		removeContext, cancelRemove := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = openConnectDockerOutput(removeContext, "rm", "--force", containerName)
		cancelRemove()
	})
	waitForOpenConnectContainerLog(t, ctx, containerName, "openconnect echo ready")
	tcpAddress := openConnectDockerPublishedAddress(t, ctx, containerName, "443/tcp")
	waitForOpenConnectTCP(t, ctx, containerName, tcpAddress)
	versionOutput, err := openConnectDockerOutput(ctx, "exec", containerName, "dpkg-query", "-W", "-f=${Version}", "ocserv")
	require.NoError(t, err)
	require.Equal(t, openConnectOcservVersion, strings.TrimSpace(versionOutput))
	return openConnectOcservContainer{
		name:                     containerName,
		tcpAddress:               tcpAddress,
		serverAddress:            openConnectLocalhostAddress(t, tcpAddress),
		certificateAuthorityPath: certificateAuthorityPath,
		passwordPath:             passwordPath,
	}
}

func openConnectDockerPublishedAddress(t *testing.T, ctx context.Context, containerName string, port string) string {
	t.Helper()
	for {
		output, err := openConnectDockerOutput(ctx, "port", containerName, port)
		if err == nil {
			address := strings.TrimSpace(output)
			_, _, splitErr := net.SplitHostPort(address)
			if splitErr == nil {
				return address
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(E.Cause(ctx.Err(), "wait for Docker published address"))
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func openConnectLocalhostAddress(t *testing.T, address string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	return net.JoinHostPort("localhost", port)
}

func waitForOpenConnectContainerLog(t *testing.T, ctx context.Context, containerName string, expected string) {
	t.Helper()
	for {
		logs, logsErr := openConnectDockerOutput(ctx, "logs", containerName)
		if logsErr == nil && strings.Contains(logs, expected) {
			return
		}
		running, inspectErr := openConnectDockerOutput(ctx, "inspect", "--format", "{{.State.Running}}", containerName)
		if inspectErr == nil && strings.TrimSpace(running) != "true" {
			t.Fatalf("ocserv container exited while waiting for %q:\n%s", expected, logs)
		}
		select {
		case <-ctx.Done():
			t.Fatal(E.Cause(ctx.Err(), "wait for ocserv container log ", expected))
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitForOpenConnectTCP(t *testing.T, ctx context.Context, containerName string, address string) {
	t.Helper()
	for {
		conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		running, inspectErr := openConnectDockerOutput(ctx, "inspect", "--format", "{{.State.Running}}", containerName)
		if inspectErr == nil && strings.TrimSpace(running) != "true" {
			logs, _ := openConnectDockerOutput(ctx, "logs", containerName)
			t.Fatalf("ocserv container exited before TCP readiness:\n%s", logs)
		}
		select {
		case <-ctx.Done():
			t.Fatal(E.Cause(ctx.Err(), "wait for ocserv TCP listener"))
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func openConnectDockerOutput(ctx context.Context, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", E.Cause(err, "docker ", strings.Join(arguments, " "), ": ", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func startOpenConnectTCPProxy(t *testing.T, target string) *openConnectTCPProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxy := &openConnectTCPProxy{
		listener:    listener,
		target:      target,
		connections: make(map[*openConnectTCPProxyConnection]struct{}),
	}
	go proxy.acceptLoop()
	t.Cleanup(proxy.close)
	return proxy
}

func (p *openConnectTCPProxy) acceptLoop() {
	for {
		downstream, err := p.listener.Accept()
		if err != nil {
			return
		}
		upstream, err := net.DialTimeout("tcp", p.target, 5*time.Second)
		if err != nil {
			_ = downstream.Close()
			continue
		}
		connection := &openConnectTCPProxyConnection{
			proxy:      p,
			downstream: downstream,
			upstream:   upstream,
		}
		p.access.Lock()
		if p.closed {
			p.access.Unlock()
			connection.close()
			return
		}
		p.connections[connection] = struct{}{}
		p.accepted.Add(1)
		p.access.Unlock()
		go connection.copy(upstream, downstream)
		go connection.copy(downstream, upstream)
	}
}

func (c *openConnectTCPProxyConnection) copy(destination net.Conn, source net.Conn) {
	_, _ = io.Copy(destination, source)
	c.close()
}

func (c *openConnectTCPProxyConnection) close() {
	c.closeOnce.Do(func() {
		_ = c.downstream.Close()
		_ = c.upstream.Close()
		c.proxy.access.Lock()
		delete(c.proxy.connections, c)
		c.proxy.access.Unlock()
	})
}

func (p *openConnectTCPProxy) dropConnections() int {
	p.access.Lock()
	connections := make([]*openConnectTCPProxyConnection, 0, len(p.connections))
	for connection := range p.connections {
		connections = append(connections, connection)
	}
	p.access.Unlock()
	for _, connection := range connections {
		connection.close()
	}
	return len(connections)
}

func (p *openConnectTCPProxy) close() {
	p.access.Lock()
	p.closed = true
	p.access.Unlock()
	_ = p.listener.Close()
	p.dropConnections()
}

func waitForOpenConnectProxyAccept(t *testing.T, proxy *openConnectTCPProxy, previous uint64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if proxy.accepted.Load() > previous {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("OpenConnect proxy accepted %d connections, expected more than %d after CSTP drop", proxy.accepted.Load(), previous)
}
