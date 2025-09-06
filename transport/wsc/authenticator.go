package wsc

import "context"

type AuthenticateParams struct {
	Auth    string
	MaxConn int
}

type AuthenticateResult struct {
	ID      int64
	Rate    int64
	MaxConn int
}

type ReportUsageParams struct {
	ID          int64
	UsedTraffic int64
}

type ReportUsageResult struct{}

type Authenticator interface {
	Authenticate(ctx context.Context, params AuthenticateParams) (AuthenticateResult, error)
	ReportUsage(ctx context.Context, params ReportUsageParams) (ReportUsageResult, error)
}
