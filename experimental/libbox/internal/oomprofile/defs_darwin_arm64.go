//go:build darwin && arm64

package oomprofile

type machVMRegionBasicInfoData struct {
	Protection     int32
	MaxProtection  int32
	Inheritance    uint32
	Shared         int32
	Reserved       int32
	Offset         [8]byte
	Behavior       int32
	UserWiredCount uint16
	PadCgo1        [2]byte
}

const (
	_VM_PROT_READ    = 0x1
	_VM_PROT_EXECUTE = 0x4

	_MACH_SEND_INVALID_DEST = 0x10000003

	_MAXPATHLEN = 0x400
)
