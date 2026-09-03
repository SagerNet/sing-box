package powerreport

/*
#include <mach/mach_time.h>
#include <net/if.h>
#include <net/if_var.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <unistd.h>

// The iOS SDK does not ship libproc.h; the symbol is exported by libSystem
// on all darwin platforms.
int proc_pid_rusage(int pid, int flavor, rusage_info_t *buffer);
*/
import "C"

import (
	"encoding/binary"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	timebaseOnce  sync.Once
	timebaseNumer int64
	timebaseDenom int64
)

func machTimebase() (int64, int64) {
	timebaseOnce.Do(func() {
		var timebase C.struct_mach_timebase_info
		C.mach_timebase_info(&timebase)
		timebaseNumer = int64(timebase.numer)
		timebaseDenom = int64(timebase.denom)
	})
	return timebaseNumer, timebaseDenom
}

func machToNano(value uint64) int64 {
	numer, denom := machTimebase()
	if denom == 0 {
		return int64(value)
	}
	return int64(value) * numer / denom
}

// The time fields of rusage_info are in mach_absolute_time units on arm64,
// not nanoseconds; the header does not document this.
func readSystemUsage() systemUsage {
	var info C.struct_rusage_info_v6
	result := C.proc_pid_rusage(C.int(os.Getpid()), C.RUSAGE_INFO_V6, (*C.rusage_info_t)(unsafe.Pointer(&info)))
	if result == 0 {
		return systemUsage{
			valid:                       true,
			userTime:                    machToNano(uint64(info.ri_user_time)),
			systemTime:                  machToNano(uint64(info.ri_system_time)),
			performanceUserTime:         machToNano(uint64(info.ri_user_ptime)),
			performanceSystemTime:       machToNano(uint64(info.ri_system_ptime)),
			qosDefaultTime:              machToNano(uint64(info.ri_cpu_time_qos_default)),
			qosMaintenanceTime:          machToNano(uint64(info.ri_cpu_time_qos_maintenance)),
			qosBackgroundTime:           machToNano(uint64(info.ri_cpu_time_qos_background)),
			qosUtilityTime:              machToNano(uint64(info.ri_cpu_time_qos_utility)),
			qosLegacyTime:               machToNano(uint64(info.ri_cpu_time_qos_legacy)),
			qosUserInitiatedTime:        machToNano(uint64(info.ri_cpu_time_qos_user_initiated)),
			qosUserInteractiveTime:      machToNano(uint64(info.ri_cpu_time_qos_user_interactive)),
			packageIdleWakeups:          uint64(info.ri_pkg_idle_wkups),
			interruptWakeups:            uint64(info.ri_interrupt_wkups),
			diskBytesWritten:            uint64(info.ri_diskio_byteswritten),
			energyNanojoules:            uint64(info.ri_energy_nj),
			performanceEnergyNanojoules: uint64(info.ri_penergy_nj),
		}
	}
	var infoV4 C.struct_rusage_info_v4
	result = C.proc_pid_rusage(C.int(os.Getpid()), C.RUSAGE_INFO_V4, (*C.rusage_info_t)(unsafe.Pointer(&infoV4)))
	if result != 0 {
		return systemUsage{}
	}
	return systemUsage{
		valid:                  true,
		userTime:               machToNano(uint64(infoV4.ri_user_time)),
		systemTime:             machToNano(uint64(infoV4.ri_system_time)),
		qosDefaultTime:         machToNano(uint64(infoV4.ri_cpu_time_qos_default)),
		qosMaintenanceTime:     machToNano(uint64(infoV4.ri_cpu_time_qos_maintenance)),
		qosBackgroundTime:      machToNano(uint64(infoV4.ri_cpu_time_qos_background)),
		qosUtilityTime:         machToNano(uint64(infoV4.ri_cpu_time_qos_utility)),
		qosLegacyTime:          machToNano(uint64(infoV4.ri_cpu_time_qos_legacy)),
		qosUserInitiatedTime:   machToNano(uint64(infoV4.ri_cpu_time_qos_user_initiated)),
		qosUserInteractiveTime: machToNano(uint64(infoV4.ri_cpu_time_qos_user_interactive)),
		packageIdleWakeups:     uint64(infoV4.ri_pkg_idle_wkups),
		interruptWakeups:       uint64(infoV4.ri_interrupt_wkups),
		diskBytesWritten:       uint64(infoV4.ri_diskio_byteswritten),
	}
}

func readClocks() (absoluteTime int64, continuousTime int64) {
	return machToNano(uint64(C.mach_absolute_time())), machToNano(uint64(C.mach_continuous_time()))
}

func readInterfaceCounters() map[string]interfaceCounters {
	table, err := syscall.RouteRIB(syscall.NET_RT_IFLIST2, 0)
	if err != nil {
		return nil
	}
	var message C.struct_if_msghdr2
	messageSize := int(unsafe.Sizeof(message))
	messageBytes := unsafe.Slice((*byte)(unsafe.Pointer(&message)), messageSize)
	result := make(map[string]interfaceCounters)
	for len(table) >= 4 {
		messageLength := int(binary.NativeEndian.Uint16(table))
		if messageLength < 4 || messageLength > len(table) {
			break
		}
		if table[3] == syscall.RTM_IFINFO2 && messageLength >= messageSize {
			copy(messageBytes, table[:messageSize])
			var nameBuffer [C.IF_NAMESIZE]C.char
			if C.if_indextoname(C.uint(message.ifm_index), &nameBuffer[0]) != nil {
				name := C.GoString(&nameBuffer[0])
				if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "pdp_ip") {
					result[name] = interfaceCounters{
						inPackets:  uint64(message.ifm_data.ifi_ipackets),
						outPackets: uint64(message.ifm_data.ifi_opackets),
						inBytes:    uint64(message.ifm_data.ifi_ibytes),
						outBytes:   uint64(message.ifm_data.ifi_obytes),
					}
				}
			}
		}
		table = table[messageLength:]
	}
	return result
}
