//go:build !with_usbip || !(linux || (darwin && cgo) || windows)

package daemon

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *StartedService) ProvideUSBDevices(server grpc.BidiStreamingServer[USBProviderMessage, USBServerMessage]) error {
	return status.Error(codes.Unimplemented, "USB/IP is not included in this build, rebuild with -tags with_usbip")
}

func (s *StartedService) SubscribeUSBIPServerStatus(_ *emptypb.Empty, server grpc.ServerStreamingServer[USBIPServerStatusUpdate]) error {
	return status.Error(codes.NotFound, "USB/IP is not included in this build, rebuild with -tags with_usbip")
}
