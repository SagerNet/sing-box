//nolint:unused
package main

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type peerIdentity struct {
	UserID           string
	ProcessID        uint32
	ProcessStartTime uint64
	SessionID        uint32
}

type peerAuthInfo struct {
	credentials.CommonAuthInfo
	identity peerIdentity
}

func (i *peerAuthInfo) AuthType() string {
	return "local-process"
}

func peerIdentityFromContext(ctx context.Context) (peerIdentity, error) {
	peerInfo, loaded := peer.FromContext(ctx)
	if !loaded || peerInfo.AuthInfo == nil {
		return platformFallbackPeerIdentity(ctx)
	}
	authInfo, loaded := peerInfo.AuthInfo.(*peerAuthInfo)
	if !loaded {
		return peerIdentity{}, E.New("unexpected peer authentication type")
	}
	return authInfo.identity, nil
}

var _ credentials.AuthInfo = (*peerAuthInfo)(nil)

type peerConnection interface {
	net.Conn
	peerConnectionIdentity() peerIdentity
}

func (d *Daemon) registerPeerConnection(connection peerConnection) {
	d.peerAccess.Lock()
	defer d.peerAccess.Unlock()
	if d.peerConnections == nil {
		d.peerConnections = make(map[peerConnection]peerIdentity)
	}
	d.peerConnections[connection] = connection.peerConnectionIdentity()
}

func (d *Daemon) unregisterPeerConnection(connection peerConnection) {
	d.peerAccess.Lock()
	defer d.peerAccess.Unlock()
	delete(d.peerConnections, connection)
}
