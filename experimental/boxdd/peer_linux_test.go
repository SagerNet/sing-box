//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLinuxPeerAuthentication(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "daemon.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	clientConnection, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConnection.Close()
	serverConnection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	daemon := &Daemon{}
	authenticatedConnection, authenticationInformation, err := (&linuxTransportCredentials{daemon: daemon}).ServerHandshake(serverConnection)
	if err != nil {
		t.Fatal(err)
	}
	authentication, loaded := authenticationInformation.(*peerAuthInfo)
	if !loaded {
		t.Fatal("missing peer authentication information")
	}
	identity := authentication.identity
	if identity.UserID != strconv.Itoa(os.Getuid()) {
		t.Fatalf("unexpected peer user ID: %s", identity.UserID)
	}
	if identity.ProcessID != uint32(os.Getpid()) {
		t.Fatalf("unexpected peer process ID: %d", identity.ProcessID)
	}
	expectedStartTime, err := linuxProcessStartTime(identity.ProcessID)
	if err != nil {
		t.Fatal(err)
	}
	if identity.ProcessStartTime != expectedStartTime {
		t.Fatalf("unexpected peer process start time: %d", identity.ProcessStartTime)
	}
	if len(daemon.peerConnections) != 1 {
		t.Fatalf("unexpected authenticated connection count: %d", len(daemon.peerConnections))
	}
	err = authenticatedConnection.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(daemon.peerConnections) != 0 {
		t.Fatalf("authenticated connection was not removed: %d", len(daemon.peerConnections))
	}
}
