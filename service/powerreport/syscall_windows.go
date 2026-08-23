package powerreport

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go

type processIOCounters struct {
	readOperationCount  uint64
	writeOperationCount uint64
	otherOperationCount uint64
	readTransferCount   uint64
	writeTransferCount  uint64
	otherTransferCount  uint64
}

// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getprocessiocounters
//sys getProcessIoCounters(process windows.Handle, ioCounters *processIOCounters) (err error) = kernel32.GetProcessIoCounters

// https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryunbiasedinterrupttime
//sys queryUnbiasedInterruptTime(unbiasedTime *uint64) (err error) = kernel32.QueryUnbiasedInterruptTime

// https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttime
//sys queryInterruptTime(interruptTime *uint64) = api-ms-win-core-realtime-l1-1-1.QueryInterruptTime
