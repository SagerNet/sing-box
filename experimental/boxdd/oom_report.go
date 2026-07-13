package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
)

const oomReportsDirectoryName = "oom_reports"

// File order and the profile classification follow the client convention:
// sing-box-for-apple Library/Shared/OOMReportManager.swift (availableFiles)
// and Library/Shared/OOMReportArchive.swift (profileFiles).
var oomReportLeadingFileOrder = []string{metadataFileName, configSnapshotFileName, goLogFileName}

func (s *desktopService) ListOOMReports(ctx context.Context, empty *emptypb.Empty) (*OOMReportList, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(reportsDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return &OOMReportList{}, nil
		}
		return nil, err
	}
	reports := make([]*OOMReportEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(reportsDirectory, entry.Name())
		if !reportOwnedBy(fullPath, userID) {
			continue
		}
		reports = append(reports, &OOMReportEntry{
			Name:       entry.Name(),
			RecordedAt: reportTime(fullPath, "recordedAt").UnixMilli(),
			IsRead:     reportIsRead(fullPath),
		})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].RecordedAt > reports[j].RecordedAt
	})
	return &OOMReportList{Reports: reports}, nil
}

func (s *desktopService) ReadOOMReport(ctx context.Context, request *OOMReportRequest) (*OOMReportContent, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	fullPath, err := reportPathForUser(reportsDirectory, request.Name, userID)
	if err != nil {
		return nil, err
	}
	files := make([]*OOMReportFile, 0, len(oomReportLeadingFileOrder))
	for _, fileName := range oomReportLeadingFileOrder {
		content, readError := os.ReadFile(filepath.Join(fullPath, fileName))
		if readError != nil {
			if os.IsNotExist(readError) {
				continue
			}
			return nil, readError
		}
		files = append(files, &OOMReportFile{
			Name:    fileName,
			Content: content,
		})
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	profileNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if slices.Contains(oomReportLeadingFileOrder, name) {
			continue
		}
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)
	for _, name := range profileNames {
		files = append(files, &OOMReportFile{
			Name:      name,
			IsProfile: true,
		})
	}
	return &OOMReportContent{Files: files}, nil
}

func (s *desktopService) MarkOOMReportRead(ctx context.Context, request *OOMReportRequest) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
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

func (s *desktopService) ExportOOMReport(ctx context.Context, request *OOMReportExportRequest) (*CrashReportArchive, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	return exportReportArchive(reportsDirectory, request.Name, userID, request.WithConfiguration, request.WithLog, request.Encrypt)
}

func (s *desktopService) DeleteOOMReport(ctx context.Context, request *OOMReportRequest) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
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

func (s *desktopService) DeleteAllOOMReports(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	reportsDirectory, userID, err := s.daemon.reportCaller(ctx, oomReportsDirectoryName)
	if err != nil {
		return nil, err
	}
	err = deleteReportsForUser(reportsDirectory, userID)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
