package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"google.golang.org/protobuf/types/known/emptypb"
)

const crashReportsDirectoryName = "crash_reports"

var crashReportFileOrder = []string{metadataFileName, nativeLogFileName, goLogFileName, configSnapshotFileName}

func (s *desktopService) ListCrashReports(ctx context.Context, empty *emptypb.Empty) (*CrashReportList, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(reportsDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return &CrashReportList{}, nil
		}
		return nil, err
	}
	reports := make([]*CrashReportEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(reportsDirectory, entry.Name())
		if !reportOwnedBy(fullPath, userID) {
			continue
		}
		reports = append(reports, &CrashReportEntry{
			Name:      entry.Name(),
			CrashedAt: reportTime(fullPath, "crashedAt").UnixMilli(),
			IsRead:    reportIsRead(fullPath),
		})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CrashedAt > reports[j].CrashedAt
	})
	return &CrashReportList{Reports: reports}, nil
}

func (s *desktopService) ReadCrashReport(ctx context.Context, request *CrashReportRequest) (*CrashReportContent, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	fullPath, err := reportPathForUser(reportsDirectory, request.Name, userID)
	if err != nil {
		return nil, err
	}
	files := make([]*CrashReportFile, 0, len(crashReportFileOrder))
	for _, fileName := range crashReportFileOrder {
		content, readError := os.ReadFile(filepath.Join(fullPath, fileName))
		if readError != nil {
			if os.IsNotExist(readError) {
				continue
			}
			return nil, readError
		}
		files = append(files, &CrashReportFile{
			Name:    fileName,
			Content: string(content),
		})
	}
	return &CrashReportContent{Files: files}, nil
}

func (s *desktopService) MarkCrashReportRead(ctx context.Context, request *CrashReportRequest) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	fullPath, err := reportPathForUser(reportsDirectory, request.Name, userID)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(fullPath, readMarkerFileName), nil, 0o600)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *desktopService) ExportCrashReport(ctx context.Context, request *CrashReportExportRequest) (*CrashReportArchive, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	return exportReportArchive(reportsDirectory, request.Name, userID, request.WithConfiguration, request.WithLog, request.Encrypt)
}

func (s *desktopService) DeleteCrashReport(ctx context.Context, request *CrashReportRequest) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	fullPath, err := reportPathForUser(reportsDirectory, request.Name, userID)
	if err != nil {
		return nil, err
	}
	err = os.RemoveAll(fullPath)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *desktopService) DeleteAllCrashReports(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, crashReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	err = deleteReportsForUser(reportsDirectory, userID)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
