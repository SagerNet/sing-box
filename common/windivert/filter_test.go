package windivert

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestRejectFilter(t *testing.T) {
	t.Parallel()
	bin, flags, err := reject().encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(bin) != filterInstBytes {
		t.Fatalf("reject filter len: got %d, want %d", len(bin), filterInstBytes)
	}
	if flags != 0 {
		t.Fatalf("reject filter flags: got %x, want 0", flags)
	}
	// word0: field=ZERO=0, test=EQ=0, success=REJECT=0x7FFF
	word0 := binary.LittleEndian.Uint32(bin[0:4])
	if word0 != uint32(resultReject)<<16 {
		t.Fatalf("reject word0 = %08x", word0)
	}
	// word1: failure=REJECT
	word1 := binary.LittleEndian.Uint32(bin[4:8])
	if word1 != uint32(resultReject) {
		t.Fatalf("reject word1 = %08x", word1)
	}
}

func TestOutboundTCPFilterIPv4(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.1.2.3:54321")
	dst := netip.MustParseAddrPort("1.2.3.4:443")
	f, err := OutboundTCP(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	bin, flags, err := f.encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := filterFlagOutbound | filterFlagIP; flags != want {
		t.Fatalf("flags: got %x, want %x", flags, want)
	}
	// 7 instructions: OUTBOUND, IP, TCP, IP_SRCADDR, IP_DSTADDR, TCP_SRCPORT, TCP_DSTPORT
	const wantInsts = 7
	if len(bin) != wantInsts*filterInstBytes {
		t.Fatalf("instruction count: got %d, want %d", len(bin)/filterInstBytes, wantInsts)
	}

	// Inst 0: OUTBOUND == 1, success=1, failure=REJECT
	checkInst(t, bin[0*filterInstBytes:], 0, fieldOutbound, testEQ, 1, resultReject, 1)
	// Inst 1: IP == 1, success=2
	checkInst(t, bin[1*filterInstBytes:], 1, fieldIP, testEQ, 2, resultReject, 1)
	// Inst 2: TCP == 1, success=3
	checkInst(t, bin[2*filterInstBytes:], 2, fieldTCP, testEQ, 3, resultReject, 1)
	// Inst 3: IP_SRCADDR == 10.1.2.3 (host-order uint32 = 0x0A010203, arg[1]=0x0000FFFF marker)
	checkInst(t, bin[3*filterInstBytes:], 3, fieldIPSrcAddr, testEQ, 4, resultReject, 0x0A010203)
	checkArg1(t, bin[3*filterInstBytes:], 3, 0x0000FFFF)
	// Inst 4: IP_DSTADDR == 1.2.3.4
	checkInst(t, bin[4*filterInstBytes:], 4, fieldIPDstAddr, testEQ, 5, resultReject, 0x01020304)
	checkArg1(t, bin[4*filterInstBytes:], 4, 0x0000FFFF)
	// Inst 5: TCP_SRCPORT == 54321
	checkInst(t, bin[5*filterInstBytes:], 5, fieldTCPSrcPort, testEQ, 6, resultReject, 54321)
	// Last inst 6: TCP_DSTPORT == 443, success=ACCEPT
	checkInst(t, bin[6*filterInstBytes:], 6, fieldTCPDstPort, testEQ, resultAccept, resultReject, 443)
}

func TestOutboundTCPFilterIPv6(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("[2001:db8::1]:54321")
	dst := netip.MustParseAddrPort("[2001:db8::2]:443")
	f, err := OutboundTCP(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	bin, flags, err := f.encode()
	if err != nil {
		t.Fatal(err)
	}
	if want := filterFlagOutbound | filterFlagIPv6; flags != want {
		t.Fatalf("flags: got %x, want %x", flags, want)
	}
	// Inst 3: IPv6_SRCADDR. The driver stores the address in reversed
	// word order: arg[0]=low (bytes 12..15)=1, arg[3]=high (bytes 0..3)=0x20010db8.
	off := 3 * filterInstBytes
	a0 := binary.LittleEndian.Uint32(bin[off+8:])
	a1 := binary.LittleEndian.Uint32(bin[off+12:])
	a2 := binary.LittleEndian.Uint32(bin[off+16:])
	a3 := binary.LittleEndian.Uint32(bin[off+20:])
	if a0 != 1 || a1 != 0 || a2 != 0 || a3 != 0x20010db8 {
		t.Fatalf("ipv6 src arg=[%08x %08x %08x %08x], want [1 0 0 0x20010db8]", a0, a1, a2, a3)
	}
}

func TestOutboundTCPFilterMixedFamily(t *testing.T) {
	t.Parallel()
	src := netip.MustParseAddrPort("10.0.0.1:1234")
	dst := netip.MustParseAddrPort("[2001:db8::1]:443")
	if _, err := OutboundTCP(src, dst); err == nil {
		t.Fatal("expected error for mixed families")
	}
}

func checkArg1(t *testing.T, raw []byte, idx int, arg1 uint32) {
	t.Helper()
	got := binary.LittleEndian.Uint32(raw[12:16])
	if got != arg1 {
		t.Errorf("inst %d arg[1]: got %08x, want %08x", idx, got, arg1)
	}
}

func checkInst(t *testing.T, raw []byte, idx int, field uint16, test uint8, success, failure uint16, arg0 uint32) {
	t.Helper()
	word0 := binary.LittleEndian.Uint32(raw[0:4])
	word1 := binary.LittleEndian.Uint32(raw[4:8])
	a0 := binary.LittleEndian.Uint32(raw[8:12])
	gotField := uint16(word0 & 0x7FF)
	gotTest := uint8((word0 >> 11) & 0x1F)
	gotSuccess := uint16(word0 >> 16)
	gotFailure := uint16(word1 & 0xFFFF)
	if gotField != field {
		t.Errorf("inst %d field: got %d, want %d", idx, gotField, field)
	}
	if gotTest != test {
		t.Errorf("inst %d test: got %d, want %d", idx, gotTest, test)
	}
	if gotSuccess != success {
		t.Errorf("inst %d success: got %d, want %d", idx, gotSuccess, success)
	}
	if gotFailure != failure {
		t.Errorf("inst %d failure: got %d, want %d", idx, gotFailure, failure)
	}
	if a0 != arg0 {
		t.Errorf("inst %d arg[0]: got %08x, want %08x", idx, a0, arg0)
	}
}
