package powerreport

import (
	"golang.org/x/sys/windows"
)

func filetimeDuration(value windows.Filetime) int64 {
	return (int64(value.HighDateTime)<<32 | int64(value.LowDateTime)) * 100
}

func readSystemUsage() systemUsage {
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	err := windows.GetProcessTimes(windows.CurrentProcess(), &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return systemUsage{}
	}
	usage := systemUsage{
		valid:      true,
		userTime:   filetimeDuration(userTime),
		systemTime: filetimeDuration(kernelTime),
	}
	var ioCounters processIOCounters
	err = getProcessIoCounters(windows.CurrentProcess(), &ioCounters)
	if err == nil {
		usage.diskBytesWritten = ioCounters.writeTransferCount
	}
	return usage
}

func readClocks() (absoluteTime int64, continuousTime int64) {
	if procQueryInterruptTime.Find() != nil {
		return 0, 0
	}
	var unbiasedTime uint64
	err := queryUnbiasedInterruptTime(&unbiasedTime)
	if err != nil {
		return 0, 0
	}
	var interruptTime uint64
	queryInterruptTime(&interruptTime)
	return int64(unbiasedTime) * 100, int64(interruptTime) * 100
}

func readInterfaceCounters() map[string]interfaceCounters {
	return nil
}
