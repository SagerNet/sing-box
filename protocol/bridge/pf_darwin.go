package bridge

import (
	"net"
	"net/netip"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

// Layouts and values mirror bsd/net/pfvar.h from xnu, which the SDKs do not
// ship; unchanged from xnu-4570.1.46 (macOS 10.13) through xnu-12377.1.9.

const (
	pfRulesetScrub  = 0
	pfRulesetFilter = 1
	pfRulesetNat    = 2

	pfActionPass  = 0
	pfActionDrop  = 1
	pfActionScrub = 2
	pfActionNat   = 4

	pfDirectionIn  = 1
	pfDirectionOut = 2

	pfAddrTypeAddressMask      = 0
	pfAddrTypeDynamicInterface = 2

	// xnu orders the route enum PF_NOPFROUTE, PF_FASTROUTE, PF_ROUTETO,
	// PF_DUPTO, PF_REPLYTO; reply-to is 4, unlike OpenBSD where it is 3.
	pfRouteActionRouteTo = 2
	pfRouteActionReplyTo = 4

	pfStateNormal = 1

	pfNatProxyPortLow  = 50001
	pfNatProxyPortHigh = 65535
)

type pfAddr [16]byte

type pfAddrWrap struct {
	Addr   pfAddr
	Mask   pfAddr
	_      uint64
	Type   uint8
	IFlags uint8
	_      [6]byte
}

type pfRuleAddr struct {
	Addr pfAddrWrap
	_    [8]byte
	Neg  uint8
	_    [7]byte
}

type pfPool struct {
	_          [2]uint64
	_          uint64
	_          [16]byte
	_          pfAddr
	TableIndex int32
	ProxyPort  [2]uint16
	PortOp     uint8
	Opts       uint8
	AF         uint8
	_          [5]byte
}

type pfRuleUserGroup struct {
	Range [2]uint32
	Op    uint8
	_     [3]byte
}

type pfRule struct {
	Src            pfRuleAddr
	Dst            pfRuleAddr
	_              [8]uint64
	Label          [64]byte
	IfName         [16]byte
	QName          [64]byte
	PQName         [64]byte
	TagName        [64]byte
	MatchTagName   [64]byte
	OverloadTable  [32]byte
	_              [2]uint64
	RPool          pfPool
	Evaluations    uint64
	Packets        [2]uint64
	Bytes          [2]uint64
	Ticket         uint64
	Owner          [64]byte
	Priority       uint32
	_              uint32
	_              [3]uint64
	OSFingerprint  uint32
	RouteTableID   uint32
	Timeout        [26]uint32
	States         uint32
	MaxStates      uint32
	SrcNodes       uint32
	MaxSrcNodes    uint32
	MaxSrcStates   uint32
	MaxSrcConn     uint32
	MaxSrcConnRate [2]uint32
	QID            uint32
	PQID           uint32
	RouteListID    uint32
	Nr             uint32
	Prob           uint32
	CreatorUID     uint32
	CreatorPID     uint32
	ReturnICMP     uint16
	ReturnICMP6    uint16
	MaxMSS         uint16
	Tag            uint16
	MatchTag       uint16
	_              uint16
	UID            pfRuleUserGroup
	GID            pfRuleUserGroup
	RuleFlag       uint32
	Action         uint8
	Direction      uint8
	Log            uint8
	LogIf          uint8
	Quick          uint8
	IfNot          uint8
	MatchTagNot    uint8
	NatPass        uint8
	KeepState      uint8
	AF             uint8
	Proto          uint8
	Type           uint8
	Code           uint8
	Flags          uint8
	FlagSet        uint8
	MinTTL         uint8
	AllowOpts      uint8
	RouteAction    uint8
	ReturnTTL      uint8
	TOS            uint8
	AnchorRelative uint8
	AnchorWildcard uint8
	Flush          uint8
	ProtoVariant   uint8
	ExtFilter      uint8
	ExtMap         uint8
	_              uint16
	DummynetPipe   uint32
	DummynetType   uint32
}

type pfPoolAddr struct {
	Addr   pfAddrWrap
	_      [2]uint64
	IfName [16]byte
	_      uint64
}

type pfiocRule struct {
	Action     uint32
	Ticket     uint32
	PoolTicket uint32
	Nr         uint32
	Anchor     [1024]byte
	AnchorCall [1024]byte
	Rule       pfRule
}

type pfiocPoolAddr struct {
	Action  uint32
	Ticket  uint32
	Nr      uint32
	RNum    uint32
	RAction uint8
	RLast   uint8
	AF      uint8
	Anchor  [1024]byte
	_       [5]byte
	Addr    pfPoolAddr
}

type pfiocTransElement struct {
	RulesetIndex int32
	Anchor       [1024]byte
	Ticket       uint32
}

type pfiocTrans struct {
	Size        int32
	ElementSize int32
	Array       *pfiocTransElement
}

type pfiocRemoveToken struct {
	Token    uint64
	RefCount uint64
}

const (
	iocParamMask = 0x1fff
	iocOut       = 0x40000000
	iocIn        = 0x80000000
	iocInOut     = iocIn | iocOut
)

const (
	diocAddRule    = iocInOut | (uint(unsafe.Sizeof(pfiocRule{}))&iocParamMask)<<16 | 'D'<<8 | 4
	diocStartRef   = iocOut | 8<<16 | 'D'<<8 | 8
	diocStopRef    = iocInOut | (uint(unsafe.Sizeof(pfiocRemoveToken{}))&iocParamMask)<<16 | 'D'<<8 | 9
	diocBeginAddrs = iocInOut | (uint(unsafe.Sizeof(pfiocPoolAddr{}))&iocParamMask)<<16 | 'D'<<8 | 51
	diocAddAddr    = iocInOut | (uint(unsafe.Sizeof(pfiocPoolAddr{}))&iocParamMask)<<16 | 'D'<<8 | 52
	diocXBegin     = iocInOut | (uint(unsafe.Sizeof(pfiocTrans{}))&iocParamMask)<<16 | 'D'<<8 | 81
	diocXCommit    = iocInOut | (uint(unsafe.Sizeof(pfiocTrans{}))&iocParamMask)<<16 | 'D'<<8 | 82
	diocXRollback  = iocInOut | (uint(unsafe.Sizeof(pfiocTrans{}))&iocParamMask)<<16 | 'D'<<8 | 83
)

type pfAnchorRule struct {
	RulesetIndex int32
	Rule         pfRule
	Pool         pfPoolAddr
}

type pfDevice struct {
	fd int
}

func openPfDevice() (*pfDevice, error) {
	fd, err := unix.Open("/dev/pf", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, E.Cause(err, "open /dev/pf")
	}
	return &pfDevice{fd: fd}, nil
}

func (d *pfDevice) Close() error {
	return unix.Close(d.fd)
}

func (d *pfDevice) ioctl(request uint, pointer unsafe.Pointer) error {
	return unixIoctlPtr(d.fd, request, pointer)
}

func (d *pfDevice) StartReference() (uint64, error) {
	var token uint64
	err := d.ioctl(uint(diocStartRef), unsafe.Pointer(&token))
	if err != nil {
		return 0, E.Cause(err, "DIOCSTARTREF")
	}
	return token, nil
}

func (d *pfDevice) StopReference(token uint64) error {
	remove := pfiocRemoveToken{Token: token}
	err := d.ioctl(uint(diocStopRef), unsafe.Pointer(&remove))
	if err != nil {
		return E.Cause(err, "DIOCSTOPREF")
	}
	return nil
}

// LoadAnchor atomically replaces the anchor's scrub, nat and filter rulesets;
// empty rules flush the anchor.
func (d *pfDevice) LoadAnchor(anchor string, rules []pfAnchorRule) error {
	elements := [3]pfiocTransElement{
		{RulesetIndex: pfRulesetScrub},
		{RulesetIndex: pfRulesetNat},
		{RulesetIndex: pfRulesetFilter},
	}
	for i := range elements {
		copy(elements[i].Anchor[:], anchor)
	}
	trans := pfiocTrans{
		Size:        int32(len(elements)),
		ElementSize: int32(unsafe.Sizeof(pfiocTransElement{})),
		Array:       &elements[0],
	}
	err := d.ioctl(uint(diocXBegin), unsafe.Pointer(&trans))
	if err != nil {
		return E.Cause(err, "DIOCXBEGIN")
	}
	for _, rule := range rules {
		err = d.addRule(anchor, &elements, rule)
		if err != nil {
			_ = d.ioctl(uint(diocXRollback), unsafe.Pointer(&trans))
			return err
		}
	}
	err = d.ioctl(uint(diocXCommit), unsafe.Pointer(&trans))
	if err != nil {
		return E.Cause(err, "DIOCXCOMMIT")
	}
	return nil
}

func (d *pfDevice) addRule(anchor string, elements *[3]pfiocTransElement, rule pfAnchorRule) error {
	var pool pfiocPoolAddr
	err := d.ioctl(uint(diocBeginAddrs), unsafe.Pointer(&pool))
	if err != nil {
		return E.Cause(err, "DIOCBEGINADDRS")
	}
	if rule.Pool != (pfPoolAddr{}) {
		pool.Addr = rule.Pool
		pool.AF = rule.Rule.AF
		err = d.ioctl(uint(diocAddAddr), unsafe.Pointer(&pool))
		if err != nil {
			return E.Cause(err, "DIOCADDADDR")
		}
	}
	var ticket uint32
	for _, element := range elements {
		if element.RulesetIndex == rule.RulesetIndex {
			ticket = element.Ticket
		}
	}
	request := pfiocRule{
		Ticket:     ticket,
		PoolTicket: pool.Ticket,
		Rule:       rule.Rule,
	}
	copy(request.Anchor[:], anchor)
	err = d.ioctl(uint(diocAddRule), unsafe.Pointer(&request))
	if err != nil {
		return E.Cause(err, "DIOCADDRULE")
	}
	return nil
}

func pfAddrOf(address netip.Addr) (result pfAddr) {
	if address.Is4() {
		addr4 := address.As4()
		copy(result[:], addr4[:])
	} else {
		addr16 := address.As16()
		copy(result[:], addr16[:])
	}
	return
}

func pfMaskOf(bits int, is4 bool) (result pfAddr) {
	totalBits := 128
	if is4 {
		totalBits = 32
	}
	copy(result[:], net.CIDRMask(bits, totalBits))
	return
}

func pfHostAddress(address netip.Addr) pfAddrWrap {
	return pfPrefixAddress(netip.PrefixFrom(address, address.BitLen()))
}

func pfPrefixAddress(prefix netip.Prefix) pfAddrWrap {
	return pfAddrWrap{
		Type: pfAddrTypeAddressMask,
		Addr: pfAddrOf(prefix.Addr()),
		Mask: pfMaskOf(prefix.Bits(), prefix.Addr().Is4()),
	}
}

func pfDynamicInterfaceAddress(interfaceName string, is4 bool) pfAddrWrap {
	wrap := pfAddrWrap{
		Type: pfAddrTypeDynamicInterface,
	}
	if is4 {
		wrap.Mask = pfMaskOf(32, true)
	} else {
		wrap.Mask = pfMaskOf(128, false)
	}
	copy(wrap.Addr[:], interfaceName)
	return wrap
}

func pfFamily(is4 bool) uint8 {
	if is4 {
		return unix.AF_INET
	}
	return unix.AF_INET6
}
