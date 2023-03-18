package settings

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func SetSystemTime(nowTime time.Time) error {
	var systemTime windows.Systemtime
	systemTime.Year = uint16(nowTime.Year())
	systemTime.Month = uint16(nowTime.Month())
	systemTime.Day = uint16(nowTime.Day())
	systemTime.Hour = uint16(nowTime.Hour())
	systemTime.Minute = uint16(nowTime.Minute())
	systemTime.Second = uint16(nowTime.Second())
	systemTime.Milliseconds = uint16(nowTime.UnixMilli() - nowTime.Unix()*1000)

	dllKernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := dllKernel32.NewProc("SetSystemTime")

	_, _, err := proc.Call(
		uintptr(unsafe.Pointer(&systemTime)),
	)

	if err != nil && err.Error() != "The operation completed successfully." {
		return err
	}

	return nil
}
