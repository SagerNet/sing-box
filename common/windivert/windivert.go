// Upstream: https://github.com/basil00/WinDivert v2.2.2, redistributed
// under its LGPL v3 option; see assets/LICENSE.txt.
package windivert

import "unsafe"

const AssetVersion = "2.2.2"

const (
	Asset64Name   = "WinDivert64.sys"
	Asset32Name   = "WinDivert32.sys"
	Asset64SHA256 = "8da085332782708d8767bcace5327a6ec7283c17cfb85e40b03cd2323a90ddc2"
	Asset32SHA256 = "2f43f4251be4d72dd56c91bf6cce475d379eb9ba6c4dda2be3022ea633d5e807"
)

// WINDIVERT_MTU_MAX from windivert.h.
const MTUMax = 40 + 0xFFFF

type Layer uint32

const LayerNetwork Layer = 0

type Flag uint64

const (
	FlagSniff    Flag = 0x0001
	FlagSendOnly Flag = 0x0008
)

const (
	PriorityHighest int16 = 30000
	PriorityLowest  int16 = -30000
)

// WINDIVERT_ADDRESS from windivert.h: bits packs Layer:8 | Event:8 | flags |
// Reserved1:8, and the trailing 64 bytes are a union of
// WINDIVERT_DATA_NETWORK / FLOW / SOCKET / REFLECT.
type Address struct {
	Timestamp int64
	bits      uint32
	Reserved2 uint32
	_         [64]byte
}

var _ [80]byte = [unsafe.Sizeof(Address{})]byte{}

const (
	addrBitOutbound    = 17
	addrBitIPv6        = 20
	addrBitIPChecksum  = 21
	addrBitTCPChecksum = 22
	addrBitUDPChecksum = 23
)

func getFlagBit(bits uint32, pos uint) bool { return bits&(1<<pos) != 0 }
func setFlagBit(bits uint32, pos uint, v bool) uint32 {
	if v {
		return bits | (1 << pos)
	}
	return bits &^ (1 << pos)
}

func (a *Address) IPv6() bool { return getFlagBit(a.bits, addrBitIPv6) }

func (a *Address) SetIPv6(v bool) {
	a.bits = setFlagBit(a.bits, addrBitIPv6, v)
}

func (a *Address) SetOutbound(v bool) {
	a.bits = setFlagBit(a.bits, addrBitOutbound, v)
}

func (a *Address) SetIPChecksum(v bool) {
	a.bits = setFlagBit(a.bits, addrBitIPChecksum, v)
}

func (a *Address) SetTCPChecksum(v bool) {
	a.bits = setFlagBit(a.bits, addrBitTCPChecksum, v)
}

func (a *Address) SetUDPChecksum(v bool) {
	a.bits = setFlagBit(a.bits, addrBitUDPChecksum, v)
}
