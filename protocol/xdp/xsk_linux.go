//go:build linux

package xdp

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

const (
	defaultFrameSize  = 4096
	defaultFrameCount = 4096
	defaultRingSize   = 2048

	ethHeaderLen = 14

	// mmap offsets for RX/TX rings
	xdpRxPgoff = int64(0)
	xdpTxPgoff = int64(0x80000000)
)

// umemRing is a ring buffer for UMEM fill/completion operations.
type umemRing struct {
	producer *uint32
	consumer *uint32
	descs    []uint64
	mask     uint32
	size     uint32
}

// xskRing is a ring buffer for RX/TX operations.
type xskRing struct {
	producer *uint32
	consumer *uint32
	descs    []unix.XDPDesc
	mask     uint32
	size     uint32
}

// XSKSocket represents an AF_XDP socket with UMEM.
type XSKSocket struct {
	fd         int
	umemArea   []byte
	fillRing   umemRing
	compRing   umemRing
	rxRing     xskRing
	txRing     xskRing
	frameSize  uint32
	frameCount uint32
	ifIndex    int
	queueID    int
}

// lpmIPv4Key is the key for BPF_MAP_TYPE_LPM_TRIE with IPv4 addresses.
// Layout must match the eBPF program's struct lpm_ipv4_key.
type lpmIPv4Key struct {
	PrefixLen uint32
	Addr      [4]byte
}

// lpmIPv6Key is the key for BPF_MAP_TYPE_LPM_TRIE with IPv6 addresses.
// Layout must match the eBPF program's struct lpm_ipv6_key.
type lpmIPv6Key struct {
	PrefixLen uint32
	Addr      [16]byte
}

// ipv6HintKey is the key for the local_ipv6_hints BPF_MAP_TYPE_HASH.
// Layout must match the eBPF program's struct ipv6_hint_key.
type ipv6HintKey struct {
	Addr [16]byte
}

// XDPProgram holds loaded eBPF XDP program and maps.
type XDPProgram struct {
	collection            *ebpf.Collection
	xdpLink               link.Link
	xsksMap               *ebpf.Map // BPF_MAP_TYPE_XSKMAP: queue_index → xsk_fd
	routeIPv4Addrs        *ebpf.Map // BPF_MAP_TYPE_LPM_TRIE: route_address IPv4
	routeIPv6Addrs        *ebpf.Map // BPF_MAP_TYPE_LPM_TRIE: route_address IPv6
	routeExcludeIPv4Addrs *ebpf.Map // BPF_MAP_TYPE_LPM_TRIE: route_exclude_address IPv4
	routeExcludeIPv6Addrs *ebpf.Map // BPF_MAP_TYPE_LPM_TRIE: route_exclude_address IPv6
	localIPv4Hints        *ebpf.Map // BPF_MAP_TYPE_HASH: local IPv4 addresses (sk_lookup gate)
	localIPv6Hints        *ebpf.Map // BPF_MAP_TYPE_HASH: local IPv6 addresses (sk_lookup gate)
}

// checkKernelVersion verifies the running kernel is new enough for XDP inbound.
// Required features and their minimum kernel versions:
//
//   - AF_XDP / BPF_MAP_TYPE_XSKMAP:                Linux 4.18
//   - bpf_sk_lookup_tcp/udp (kernel socket guard):  Linux 5.2
//   - BPF_LINK_TYPE_XDP (link-based XDP attach):    Linux 5.9
//
// The binding minimum is therefore 5.9.
func checkKernelVersion() error {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return fmt.Errorf("read kernel version: %w", err)
	}
	release := string(bytes.TrimSpace(data))

	var major, minor int
	if _, err := fmt.Sscanf(release, "%d.%d", &major, &minor); err != nil {
		return fmt.Errorf("parse kernel version %q: %w", release, err)
	}

	const minMajor, minMinor = 5, 9
	if major < minMajor || (major == minMajor && minor < minMinor) {
		return fmt.Errorf(
			"kernel %s is too old for XDP inbound (minimum required: %d.%d); "+
				"BPF_LINK_TYPE_XDP and bpf_sk_lookup require Linux 5.9+",
			release, minMajor, minMinor,
		)
	}
	return nil
}

// LoadXDPProgram loads the embedded xdp_prog.o and attaches to the given interface.
func LoadXDPProgram(xdpProgData []byte, ifIndex int) (*XDPProgram, error) {
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(xdpProgData))
	if err != nil {
		return nil, fmt.Errorf("load XDP collection spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("create XDP collection: %w", err)
	}

	prog := coll.Programs["xsk_def_prog"]
	if prog == nil {
		coll.Close()
		return nil, fmt.Errorf("XDP program 'xsk_def_prog' not found in embedded data")
	}

	xdpLink, err := link.AttachXDP(link.XDPOptions{
		Program:   prog,
		Interface: ifIndex,
	})
	if err != nil {
		coll.Close()
		return nil, fmt.Errorf("attach XDP program to ifindex %d: %w", ifIndex, err)
	}

	xp := &XDPProgram{
		collection:            coll,
		xdpLink:               xdpLink,
		xsksMap:               coll.Maps["xsks_map"],
		routeIPv4Addrs:        coll.Maps["route_ipv4_addrs"],
		routeIPv6Addrs:        coll.Maps["route_ipv6_addrs"],
		routeExcludeIPv4Addrs: coll.Maps["route_exclude_ipv4_addrs"],
		routeExcludeIPv6Addrs: coll.Maps["route_exclude_ipv6_addrs"],
		localIPv4Hints:        coll.Maps["local_ipv4_hints"],
		localIPv6Hints:        coll.Maps["local_ipv6_hints"],
	}

	if xp.xsksMap == nil {
		xp.Close()
		return nil, fmt.Errorf("xsks_map not found in embedded XDP program")
	}

	return xp, nil
}

// RegisterXSK registers an AF_XDP socket fd into the xsks_map for a queue.
func (xp *XDPProgram) RegisterXSK(queueID int, fd int) error {
	key := uint32(queueID)
	value := uint32(fd)
	return xp.xsksMap.Put(key, value)
}

// AddLocalIPHint adds an IP address to the local hints map (sk_lookup gate).
func (xp *XDPProgram) AddLocalIPHint(addr netip.Addr) error {
	value := uint8(1)
	if addr.Is4() {
		if xp.localIPv4Hints == nil {
			return nil
		}
		key := addr.As4()
		return xp.localIPv4Hints.Put(key, value)
	}
	if xp.localIPv6Hints == nil {
		return nil
	}
	key := ipv6HintKey{Addr: addr.As16()}
	return xp.localIPv6Hints.Put(key, value)
}

// DeleteLocalIPHint removes an IP address from the local hints map.
func (xp *XDPProgram) DeleteLocalIPHint(addr netip.Addr) error {
	if addr.Is4() {
		if xp.localIPv4Hints == nil {
			return nil
		}
		key := addr.As4()
		return xp.localIPv4Hints.Delete(key)
	}
	if xp.localIPv6Hints == nil {
		return nil
	}
	key := ipv6HintKey{Addr: addr.As16()}
	return xp.localIPv6Hints.Delete(key)
}

// AddRoutePrefix adds a CIDR prefix to the route_address LPM trie (whitelist).
func (xp *XDPProgram) AddRoutePrefix(prefix netip.Prefix) error {
	addr := prefix.Addr()
	value := uint8(1)
	if addr.Is4() {
		if xp.routeIPv4Addrs == nil {
			return nil
		}
		key := lpmIPv4Key{PrefixLen: uint32(prefix.Bits()), Addr: addr.As4()}
		return xp.routeIPv4Addrs.Put(key, value)
	}
	if xp.routeIPv6Addrs == nil {
		return nil
	}
	key := lpmIPv6Key{PrefixLen: uint32(prefix.Bits()), Addr: addr.As16()}
	return xp.routeIPv6Addrs.Put(key, value)
}

// AddRouteExcludePrefix adds a CIDR prefix to the route_exclude_address LPM trie (blacklist).
func (xp *XDPProgram) AddRouteExcludePrefix(prefix netip.Prefix) error {
	addr := prefix.Addr()
	value := uint8(1)
	if addr.Is4() {
		if xp.routeExcludeIPv4Addrs == nil {
			return nil
		}
		key := lpmIPv4Key{PrefixLen: uint32(prefix.Bits()), Addr: addr.As4()}
		return xp.routeExcludeIPv4Addrs.Put(key, value)
	}
	if xp.routeExcludeIPv6Addrs == nil {
		return nil
	}
	key := lpmIPv6Key{PrefixLen: uint32(prefix.Bits()), Addr: addr.As16()}
	return xp.routeExcludeIPv6Addrs.Put(key, value)
}

func (xp *XDPProgram) Close() error {
	if xp.xdpLink != nil {
		xp.xdpLink.Close()
	}
	if xp.collection != nil {
		xp.collection.Close()
	}
	return nil
}

// NewXSKSocket creates a new AF_XDP socket bound to the given interface and queue.
func NewXSKSocket(ifIndex, queueID int, frameSize, frameCount uint32) (*XSKSocket, error) {
	if frameSize == 0 {
		frameSize = defaultFrameSize
	}
	if frameCount == 0 {
		frameCount = defaultFrameCount
	}
	ringSize := uint32(defaultRingSize)

	// Create AF_XDP socket
	fd, err := unix.Socket(unix.AF_XDP, unix.SOCK_RAW, 0)
	if err != nil {
		return nil, fmt.Errorf("create AF_XDP socket: %w", err)
	}

	xsk := &XSKSocket{
		fd:         fd,
		frameSize:  frameSize,
		frameCount: frameCount,
		ifIndex:    ifIndex,
		queueID:    queueID,
	}

	// Allocate UMEM area
	umemSize := int(frameSize * frameCount)
	xsk.umemArea, err = unix.Mmap(-1, 0, umemSize,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_PRIVATE|unix.MAP_ANONYMOUS|unix.MAP_POPULATE)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("mmap UMEM area (%d bytes): %w", umemSize, err)
	}

	// Register UMEM
	umemReg := unix.XDPUmemReg{
		Addr:     uint64(uintptr(unsafe.Pointer(&xsk.umemArea[0]))),
		Len:      uint64(umemSize),
		Size:     frameSize,
		Headroom: 0,
	}
	_, _, errno := unix.Syscall6(
		unix.SYS_SETSOCKOPT,
		uintptr(fd),
		unix.SOL_XDP,
		unix.XDP_UMEM_REG,
		uintptr(unsafe.Pointer(&umemReg)),
		unsafe.Sizeof(umemReg),
		0,
	)
	if errno != 0 {
		xsk.Close()
		return nil, fmt.Errorf("register UMEM: %w", errno)
	}

	// Set ring sizes
	if err := setRingSize(fd, unix.XDP_UMEM_FILL_RING, ringSize); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("set fill ring size: %w", err)
	}
	if err := setRingSize(fd, unix.XDP_UMEM_COMPLETION_RING, ringSize); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("set completion ring size: %w", err)
	}
	if err := setRingSize(fd, unix.XDP_RX_RING, ringSize); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("set RX ring size: %w", err)
	}
	if err := setRingSize(fd, unix.XDP_TX_RING, ringSize); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("set TX ring size: %w", err)
	}

	// Get mmap offsets
	offsets, err := getXDPMmapOffsets(fd)
	if err != nil {
		xsk.Close()
		return nil, fmt.Errorf("get mmap offsets: %w", err)
	}

	// Mmap rings
	if err := xsk.mmapFillRing(ringSize, offsets); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("mmap fill ring: %w", err)
	}
	if err := xsk.mmapCompRing(ringSize, offsets); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("mmap completion ring: %w", err)
	}
	if err := xsk.mmapRXRing(ringSize, offsets); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("mmap RX ring: %w", err)
	}
	if err := xsk.mmapTXRing(ringSize, offsets); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("mmap TX ring: %w", err)
	}

	// Populate fill ring with initial frames
	for i := uint32(0); i < ringSize; i++ {
		xsk.fillRing.descs[i] = uint64(i * frameSize)
	}
	*xsk.fillRing.producer = ringSize

	// Bind to interface and queue
	sa := unix.SockaddrXDP{
		Flags:   0,
		Ifindex: uint32(ifIndex),
		QueueID: uint32(queueID),
	}
	if err := unix.Bind(fd, &sa); err != nil {
		xsk.Close()
		return nil, fmt.Errorf("bind AF_XDP to ifindex=%d queue=%d: %w", ifIndex, queueID, err)
	}

	return xsk, nil
}

// FD returns the socket file descriptor.
func (xsk *XSKSocket) FD() int {
	return xsk.fd
}

// Receive reads available frames from the RX ring.
// Returns slices of the UMEM area containing received ethernet frames.
// The caller must call FreeRXFrames after processing.
func (xsk *XSKSocket) Receive() ([][]byte, []uint64) {
	cons := *xsk.rxRing.consumer
	prod := *xsk.rxRing.producer

	if cons == prod {
		return nil, nil
	}

	var frames [][]byte
	var addrs []uint64

	for cons != prod {
		idx := cons & xsk.rxRing.mask
		desc := xsk.rxRing.descs[idx]
		frame := xsk.umemArea[desc.Addr : desc.Addr+uint64(desc.Len)]
		frames = append(frames, frame)
		addrs = append(addrs, desc.Addr)
		cons++
	}

	*xsk.rxRing.consumer = cons
	return frames, addrs
}

// FreeRXFrames returns consumed frame addresses back to the fill ring.
func (xsk *XSKSocket) FreeRXFrames(addrs []uint64) {
	prod := *xsk.fillRing.producer

	for _, addr := range addrs {
		idx := prod & xsk.fillRing.mask
		xsk.fillRing.descs[idx] = addr
		prod++
	}

	*xsk.fillRing.producer = prod
}

// Transmit queues a frame for transmission via the TX ring.
func (xsk *XSKSocket) Transmit(data []byte) bool {
	prod := *xsk.txRing.producer
	cons := *xsk.txRing.consumer

	// Check if TX ring is full
	if prod-cons >= xsk.txRing.size {
		return false
	}

	// Find a free frame from completion ring
	addr, ok := xsk.reclaimCompFrame()
	if !ok {
		// Use a frame from the upper half of UMEM (reserved for TX)
		txFrameBase := uint64(xsk.frameCount/2) * uint64(xsk.frameSize)
		txFrameCount := xsk.frameCount / 2
		txIdx := prod % txFrameCount
		addr = txFrameBase + uint64(txIdx)*uint64(xsk.frameSize)
	}

	// Copy data into UMEM frame
	copy(xsk.umemArea[addr:addr+uint64(len(data))], data)

	idx := prod & xsk.txRing.mask
	xsk.txRing.descs[idx] = unix.XDPDesc{
		Addr: addr,
		Len:  uint32(len(data)),
	}

	*xsk.txRing.producer = prod + 1
	return true
}

// FlushTX triggers the kernel to send queued TX frames.
func (xsk *XSKSocket) FlushTX() error {
	_, err := unix.Write(xsk.fd, nil)
	if err != nil && err != unix.EAGAIN {
		// sendto with MSG_DONTWAIT
		return unix.Sendto(xsk.fd, nil, unix.MSG_DONTWAIT, &unix.SockaddrXDP{
			Ifindex: uint32(xsk.ifIndex),
			QueueID: uint32(xsk.queueID),
		})
	}
	return nil
}

// Poll waits for RX data to be available. Returns true if data is ready.
func (xsk *XSKSocket) Poll(timeoutMs int) bool {
	fds := []unix.PollFd{
		{
			Fd:     int32(xsk.fd),
			Events: unix.POLLIN,
		},
	}
	n, _ := unix.Poll(fds, timeoutMs)
	return n > 0
}

func (xsk *XSKSocket) Close() error {
	if xsk.fd > 0 {
		unix.Close(xsk.fd)
		xsk.fd = -1
	}
	if xsk.umemArea != nil {
		unix.Munmap(xsk.umemArea)
		xsk.umemArea = nil
	}
	return nil
}

// reclaimCompFrame reclaims a completed TX frame from the completion ring.
func (xsk *XSKSocket) reclaimCompFrame() (uint64, bool) {
	cons := *xsk.compRing.consumer
	prod := *xsk.compRing.producer

	if cons == prod {
		return 0, false
	}

	idx := cons & xsk.compRing.mask
	addr := xsk.compRing.descs[idx]
	*xsk.compRing.consumer = cons + 1
	return addr, true
}

func setRingSize(fd int, optName int, size uint32) error {
	return os.NewSyscallError("setsockopt", unix.SetsockoptInt(fd, unix.SOL_XDP, optName, int(size)))
}

func getXDPMmapOffsets(fd int) (*unix.XDPMmapOffsets, error) {
	var offsets unix.XDPMmapOffsets
	offsetsLen := uint32(unsafe.Sizeof(offsets))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		uintptr(fd),
		unix.SOL_XDP,
		unix.XDP_MMAP_OFFSETS,
		uintptr(unsafe.Pointer(&offsets)),
		uintptr(unsafe.Pointer(&offsetsLen)),
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return &offsets, nil
}

func (xsk *XSKSocket) mmapFillRing(size uint32, offsets *unix.XDPMmapOffsets) error {
	mapSize := offsets.Fr.Desc + uint64(size)*uint64(unsafe.Sizeof(uint64(0)))
	data, err := unix.Mmap(xsk.fd, unix.XDP_UMEM_PGOFF_FILL_RING, int(mapSize),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		return err
	}
	xsk.fillRing.producer = (*uint32)(unsafe.Pointer(&data[offsets.Fr.Producer]))
	xsk.fillRing.consumer = (*uint32)(unsafe.Pointer(&data[offsets.Fr.Consumer]))
	xsk.fillRing.size = size
	xsk.fillRing.mask = size - 1
	descsPtr := unsafe.Pointer(&data[offsets.Fr.Desc])
	xsk.fillRing.descs = unsafe.Slice((*uint64)(descsPtr), size)
	return nil
}

func (xsk *XSKSocket) mmapCompRing(size uint32, offsets *unix.XDPMmapOffsets) error {
	mapSize := offsets.Cr.Desc + uint64(size)*uint64(unsafe.Sizeof(uint64(0)))
	data, err := unix.Mmap(xsk.fd, unix.XDP_UMEM_PGOFF_COMPLETION_RING, int(mapSize),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		return err
	}
	xsk.compRing.producer = (*uint32)(unsafe.Pointer(&data[offsets.Cr.Producer]))
	xsk.compRing.consumer = (*uint32)(unsafe.Pointer(&data[offsets.Cr.Consumer]))
	xsk.compRing.size = size
	xsk.compRing.mask = size - 1
	descsPtr := unsafe.Pointer(&data[offsets.Cr.Desc])
	xsk.compRing.descs = unsafe.Slice((*uint64)(descsPtr), size)
	return nil
}

func (xsk *XSKSocket) mmapRXRing(size uint32, offsets *unix.XDPMmapOffsets) error {
	mapSize := offsets.Rx.Desc + uint64(size)*uint64(unsafe.Sizeof(unix.XDPDesc{}))
	data, err := unix.Mmap(xsk.fd, xdpRxPgoff, int(mapSize),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		return err
	}
	xsk.rxRing.producer = (*uint32)(unsafe.Pointer(&data[offsets.Rx.Producer]))
	xsk.rxRing.consumer = (*uint32)(unsafe.Pointer(&data[offsets.Rx.Consumer]))
	xsk.rxRing.size = size
	xsk.rxRing.mask = size - 1
	descsPtr := unsafe.Pointer(&data[offsets.Rx.Desc])
	xsk.rxRing.descs = unsafe.Slice((*unix.XDPDesc)(descsPtr), size)
	return nil
}

func (xsk *XSKSocket) mmapTXRing(size uint32, offsets *unix.XDPMmapOffsets) error {
	mapSize := offsets.Tx.Desc + uint64(size)*uint64(unsafe.Sizeof(unix.XDPDesc{}))
	data, err := unix.Mmap(xsk.fd, xdpTxPgoff, int(mapSize),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		return err
	}
	xsk.txRing.producer = (*uint32)(unsafe.Pointer(&data[offsets.Tx.Producer]))
	xsk.txRing.consumer = (*uint32)(unsafe.Pointer(&data[offsets.Tx.Consumer]))
	xsk.txRing.size = size
	xsk.txRing.mask = size - 1
	descsPtr := unsafe.Pointer(&data[offsets.Tx.Desc])
	xsk.txRing.descs = unsafe.Slice((*unix.XDPDesc)(descsPtr), size)
	return nil
}
