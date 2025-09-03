package wsc

import (
	"net/netip"
	"slices"
	"testing"
)

func TestPacketPayload(t *testing.T) {
	text := "TEST DATA"

	payload := packetConnPayload{
		addrPort: netip.MustParseAddrPort("9.9.9.9:53"),
		payload:  []byte(text),
	}

	bin, err := payload.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(bin) != packetConnPayloadHeaderLen+len(text) {
		t.Fatal("wrong marshal")
	}

	p2 := packetConnPayload{}
	if err := p2.UnmarshalBinary(bin); err != nil {
		t.Fatal(err)
	}

	if p2.addrPort.Port() != payload.addrPort.Port() || p2.addrPort.Addr().As16() != payload.addrPort.Addr().As16() {
		t.Fatal("failed to unmarshal addrport")
	}

	if !slices.Equal(p2.payload, payload.payload) {
		t.Fatal("unmarshaled payload not equal")
	}
}
