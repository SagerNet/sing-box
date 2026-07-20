package main

import (
	"context"
	"time"

	"github.com/sagernet/sing-box/common/networkquality"
	"github.com/sagernet/sing-box/common/stun"
	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/experimental/libbox"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ ApplicationServiceServer = (*applicationService)(nil)

type applicationService struct {
	UnimplementedApplicationServiceServer
	startedService *daemon.StartedService
}

func (s *applicationService) CheckConfig(ctx context.Context, request *ConfigContent) (*emptypb.Empty, error) {
	err := s.startedService.CheckConfig(ctx, request.Content)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *applicationService) FormatConfig(ctx context.Context, request *ConfigContent) (*ConfigContent, error) {
	content, err := s.startedService.FormatConfig(ctx, request.Content)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &ConfigContent{Content: content}, nil
}

func (s *applicationService) EncodeProfile(ctx context.Context, request *ProfileContent) (*ProfileData, error) {
	content := libbox.ProfileContent{
		Name:               request.Name,
		Type:               int32(request.Type),
		Config:             request.Config,
		RemotePath:         request.RemotePath,
		AutoUpdate:         request.AutoUpdate,
		AutoUpdateInterval: request.AutoUpdateInterval,
		LastUpdated:        request.LastUpdated,
	}
	return &ProfileData{Data: content.Encode()}, nil
}

func (s *applicationService) DecodeProfile(ctx context.Context, request *ProfileData) (*ProfileContent, error) {
	content, err := libbox.DecodeProfileContent(request.Data)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &ProfileContent{
		Type:               ProfileContent_Type(content.Type),
		Name:               content.Name,
		Config:             content.Config,
		RemotePath:         content.RemotePath,
		AutoUpdate:         content.AutoUpdate,
		AutoUpdateInterval: content.AutoUpdateInterval,
		LastUpdated:        content.LastUpdated,
	}, nil
}

func (s *applicationService) ArchiveReport(ctx context.Context, request *ArchiveReportRequest) (*emptypb.Empty, error) {
	err := libbox.CreateZipArchive(request.SourcePath, request.DestinationPath, request.Encrypt)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *applicationService) StartStandaloneNetworkQualityTest(
	request *StandaloneNetworkQualityTestRequest,
	server grpc.ServerStreamingServer[daemon.NetworkQualityTestProgress],
) error {
	httpClient := networkquality.NewHTTPClient(nil)
	defer httpClient.CloseIdleConnections()

	measurementClientFactory, err := networkquality.NewOptionalHTTP3Factory(nil, request.Http3)
	if err != nil {
		return err
	}

	result, err := networkquality.Run(networkquality.Options{
		ConfigURL:            request.ConfigUrl,
		HTTPClient:           httpClient,
		NewMeasurementClient: measurementClientFactory,
		Serial:               request.Serial,
		MaxRuntime:           time.Duration(request.MaxRuntimeSeconds) * time.Second,
		Context:              server.Context(),
		OnProgress: func(progress networkquality.Progress) {
			_ = server.Send(daemon.NewNetworkQualityTestProgress(progress))
		},
	})
	if err != nil {
		return server.Send(&daemon.NetworkQualityTestProgress{
			IsFinal: true,
			Error:   err.Error(),
		})
	}
	return server.Send(daemon.NewNetworkQualityTestResult(result))
}

func (s *applicationService) StartStandaloneSTUNTest(
	request *StandaloneSTUNTestRequest,
	server grpc.ServerStreamingServer[daemon.STUNTestProgress],
) error {
	result, err := stun.Run(stun.Options{
		Server:  request.Server,
		Context: server.Context(),
		OnProgress: func(progress stun.Progress) {
			_ = server.Send(daemon.NewSTUNTestProgress(progress))
		},
	})
	if err != nil {
		return server.Send(&daemon.STUNTestProgress{
			IsFinal: true,
			Error:   err.Error(),
		})
	}
	return server.Send(daemon.NewSTUNTestResult(result))
}
