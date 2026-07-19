//go:build linux

package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type linuxTransportCredentials struct {
	daemon *Daemon
}

type linuxAuthenticatedConnection struct {
	net.Conn
	daemon     *Daemon
	identity   peerIdentity
	close      sync.Once
	closeError error
}

func platformServerOptions(daemon *Daemon) ([]grpc.ServerOption, error) {
	if listenAddress != "" {
		return nil, nil
	}
	return []grpc.ServerOption{grpc.Creds(&linuxTransportCredentials{daemon: daemon})}, nil
}

func platformFallbackPeerIdentity(ctx context.Context) (peerIdentity, error) {
	if listenAddress != "" {
		return peerIdentity{UserID: "local"}, nil
	}
	return peerIdentity{}, E.New("missing Linux peer authentication")
}

func (c *linuxTransportCredentials) ClientHandshake(ctx context.Context, authority string, rawConnection net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, E.New("Linux local process credentials do not support client handshakes")
}

func (c *linuxTransportCredentials) ServerHandshake(rawConnection net.Conn) (net.Conn, credentials.AuthInfo, error) {
	identity, err := linuxPeerIdentity(rawConnection)
	if err != nil {
		return nil, nil, err
	}
	connection := &linuxAuthenticatedConnection{
		Conn:     rawConnection,
		daemon:   c.daemon,
		identity: identity,
	}
	c.daemon.registerPeerConnection(connection)
	authenticationInformation := &peerAuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		identity:       identity,
	}
	return connection, authenticationInformation, nil
}

func (c *linuxTransportCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "linux-local-process",
		SecurityVersion:  "1",
	}
}

func (c *linuxTransportCredentials) Clone() credentials.TransportCredentials {
	return &linuxTransportCredentials{daemon: c.daemon}
}

func (c *linuxTransportCredentials) OverrideServerName(serverNameOverride string) error {
	return nil
}

func linuxPeerIdentity(connection net.Conn) (peerIdentity, error) {
	syscallConnection, loaded := connection.(syscall.Conn)
	if !loaded {
		return peerIdentity{}, E.New("daemon endpoint does not expose a syscall connection")
	}
	rawConnection, err := syscallConnection.SyscallConn()
	if err != nil {
		return peerIdentity{}, E.Cause(err, "access daemon endpoint")
	}
	var peerCredentials *unix.Ucred
	var credentialError error
	err = rawConnection.Control(func(fileDescriptor uintptr) {
		peerCredentials, credentialError = unix.GetsockoptUcred(int(fileDescriptor), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return peerIdentity{}, E.Cause(err, "inspect daemon endpoint")
	}
	if credentialError != nil {
		return peerIdentity{}, E.Cause(credentialError, "identify daemon peer")
	}
	if peerCredentials == nil || peerCredentials.Pid <= 0 {
		return peerIdentity{}, E.New("daemon peer has invalid credentials")
	}
	processID := uint32(peerCredentials.Pid)
	processStartTime, err := linuxProcessStartTime(processID)
	if err != nil {
		return peerIdentity{}, E.Cause(err, "identify daemon peer process")
	}
	return peerIdentity{
		UserID:           strconv.FormatUint(uint64(peerCredentials.Uid), 10),
		ProcessID:        processID,
		ProcessStartTime: processStartTime,
	}, nil
}

func linuxProcessStartTime(processID uint32) (uint64, error) {
	content, err := os.ReadFile("/proc/" + strconv.FormatUint(uint64(processID), 10) + "/stat")
	if err != nil {
		return 0, err
	}
	commandEnd := bytes.LastIndexByte(content, ')')
	if commandEnd < 0 {
		return 0, E.New("invalid process stat")
	}
	fields := strings.Fields(string(content[commandEnd+1:]))
	if len(fields) <= 19 {
		return 0, E.New("incomplete process stat")
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, E.Cause(err, "parse process start time")
	}
	return startTime, nil
}

func (c *linuxAuthenticatedConnection) peerConnectionIdentity() peerIdentity {
	return c.identity
}

func (c *linuxAuthenticatedConnection) Close() error {
	c.close.Do(func() {
		c.daemon.unregisterPeerConnection(c)
		c.closeError = c.Conn.Close()
	})
	return c.closeError
}

var (
	_ credentials.TransportCredentials = (*linuxTransportCredentials)(nil)
	_ peerConnection                   = (*linuxAuthenticatedConnection)(nil)
)
