//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"strconv"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/godbus/dbus/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	policyKitService              = "org.freedesktop.PolicyKit1"
	policyKitAuthorityPath        = dbus.ObjectPath("/org/freedesktop/PolicyKit1/Authority")
	policyKitAuthorityInterface   = "org.freedesktop.PolicyKit1.Authority"
	policyKitTakeOverAction       = "io.nekohasekai.sfl.take-over-service"
	policyKitAllowUserInteraction = uint32(1)
)

type policyKitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

type policyKitAuthorizationResult struct {
	Authorized bool
	Challenge  bool
	Details    map[string]string
}

func authorizeTakeOver(ctx context.Context, identity peerIdentity) error {
	if listenAddress != "" {
		return nil
	}
	userID, err := strconv.ParseUint(identity.UserID, 10, 32)
	if err != nil || userID > math.MaxInt32 {
		return status.Error(codes.Unauthenticated, "daemon peer has an invalid Linux user ID")
	}
	if identity.ProcessID == 0 || identity.ProcessStartTime == 0 {
		return status.Error(codes.Unauthenticated, "daemon peer has an invalid Linux process identity")
	}
	cancellationContent := make([]byte, 16)
	_, err = rand.Read(cancellationContent)
	if err != nil {
		return E.Cause(err, "create PolicyKit cancellation ID")
	}
	cancellationID := "sing-box-" + hex.EncodeToString(cancellationContent)
	connection, err := dbus.ConnectSystemBus()
	if err != nil {
		return E.Cause(err, "connect to system bus")
	}
	defer connection.Close()
	authority := connection.Object(policyKitService, policyKitAuthorityPath)
	subject := policyKitSubject{
		Kind: "unix-process",
		Details: map[string]dbus.Variant{
			"pid":        dbus.MakeVariant(identity.ProcessID),
			"start-time": dbus.MakeVariant(identity.ProcessStartTime),
			"uid":        dbus.MakeVariant(int32(userID)),
		},
	}
	resultChannel := make(chan *dbus.Call, 1)
	authority.Go(
		policyKitAuthorityInterface+".CheckAuthorization",
		0,
		resultChannel,
		subject,
		policyKitTakeOverAction,
		map[string]string{},
		policyKitAllowUserInteraction,
		cancellationID,
	)
	select {
	case call := <-resultChannel:
		if call.Err != nil {
			return E.Cause(call.Err, "check PolicyKit authorization")
		}
		var result policyKitAuthorizationResult
		err = call.Store(&result)
		if err != nil {
			return E.Cause(err, "read PolicyKit authorization result")
		}
		if !result.Authorized {
			return status.Error(codes.PermissionDenied, "take over authorization was denied")
		}
		return nil
	case <-ctx.Done():
		_ = authority.Call(
			policyKitAuthorityInterface+".CancelCheckAuthorization",
			0,
			cancellationID,
		).Err
		return status.Error(codes.Canceled, "take over authorization was canceled")
	}
}
