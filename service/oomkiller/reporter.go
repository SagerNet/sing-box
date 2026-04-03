package oomkiller

type OOMReporter interface {
	WriteReport(memoryUsage uint64) error
}
