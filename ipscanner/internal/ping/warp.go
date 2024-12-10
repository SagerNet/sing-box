package ping

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"time"

	"github.com/flynn/noise"
	"github.com/sagernet/sing-box/ipscanner/internal/statute"
	"github.com/sagernet/sing-box/warp"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
)

type WarpPingResult struct {
	AddrPort netip.AddrPort
	RTT      time.Duration
	Err      error
}

func (h *WarpPingResult) Result() statute.IPInfo {
	return statute.IPInfo{AddrPort: h.AddrPort, RTT: h.RTT, CreatedAt: time.Now()}
}

func (h *WarpPingResult) Error() error {
	return h.Err
}

func (h *WarpPingResult) String() string {
	if h.Err != nil {
		return fmt.Sprintf("%s", h.Err)
	} else {
		return fmt.Sprintf("%s: protocol=%s, time=%d ms", h.AddrPort, "warp", h.RTT)
	}
}

type WarpPing struct {
	PrivateKey    string
	PeerPublicKey string
	PresharedKey  string
	IP            netip.Addr

	opts statute.ScannerOptions
}

func (h *WarpPing) Ping() statute.IPingResult {
	return h.PingContext(context.Background())
}

func (h *WarpPing) PingContext(ctx context.Context) statute.IPingResult {
	var port uint16 = 0
	if h.opts.Port == 0 || h.opts.Port == 443 {
		port = warp.RandomWarpPort()
	} else {
		port = h.opts.Port
	}
	addr := netip.AddrPortFrom(h.IP, port)
	rtt, err := initiateHandshake(
		ctx,
		addr,
		h.PrivateKey,
		h.PeerPublicKey,
		h.PresharedKey,
	)
	if err != nil {
		return h.errorResult(err)
	}

	return &WarpPingResult{AddrPort: addr, RTT: rtt, Err: nil}
}

func (h *WarpPing) errorResult(err error) *WarpPingResult {
	r := &WarpPingResult{}
	r.Err = err
	return r
}

func uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return b
}

func staticKeypair(privateKeyBase64 string) (noise.DHKey, error) {
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return noise.DHKey{}, err
	}

	var pubkey, privkey [32]byte
	copy(privkey[:], privateKey)
	curve25519.ScalarBaseMult(&pubkey, &privkey)

	return noise.DHKey{
		Private: privateKey,
		Public:  pubkey[:],
	}, nil
}

func ephemeralKeypair() (noise.DHKey, error) {
	// Generate an ephemeral private key
	ephemeralPrivateKey := make([]byte, 32)
	if _, err := rand.Read(ephemeralPrivateKey); err != nil {
		return noise.DHKey{}, err
	}

	// Derive the corresponding ephemeral public key
	ephemeralPublicKey, err := curve25519.X25519(ephemeralPrivateKey, curve25519.Basepoint)
	if err != nil {
		return noise.DHKey{}, err
	}

	return noise.DHKey{
		Private: ephemeralPrivateKey,
		Public:  ephemeralPublicKey,
	}, nil
}

func randomInt(min, max uint64) uint64 {
	rangee := max - min
	if rangee < 1 {
		return 0
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(rangee)))
	if err != nil {
		panic(err)
	}

	return min + n.Uint64()
}

func initiateHandshake(ctx context.Context, serverAddr netip.AddrPort, privateKeyBase64, peerPublicKeyBase64, presharedKeyBase64 string) (time.Duration, error) {
	staticKeyPair, err := staticKeypair(privateKeyBase64)
	if err != nil {
		return 0, err
	}

	peerPublicKey, err := base64.StdEncoding.DecodeString(peerPublicKeyBase64)
	if err != nil {
		return 0, err
	}

	presharedKey, err := base64.StdEncoding.DecodeString(presharedKeyBase64)
	if err != nil {
		return 0, err
	}

	if presharedKeyBase64 == "" {
		presharedKey = make([]byte, 32)
	}

	ephemeral, err := ephemeralKeypair()
	if err != nil {
		return 0, err
	}

	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:           cs,
		Pattern:               noise.HandshakeIK,
		Initiator:             true,
		StaticKeypair:         staticKeyPair,
		PeerStatic:            peerPublicKey,
		Prologue:              []byte("WireGuard v1 zx2c4 Jason@zx2c4.com"),
		PresharedKey:          presharedKey,
		PresharedKeyPlacement: 2,
		EphemeralKeypair:      ephemeral,
		Random:                rand.Reader,
	})
	if err != nil {
		return 0, err
	}

	// Prepare handshake initiation packet

	// TAI64N timestamp calculation
	now := time.Now().UTC()
	epochOffset := int64(4611686018427387914) // TAI offset from Unix epoch

	tai64nTimestampBuf := make([]byte, 0, 16)
	tai64nTimestampBuf = binary.BigEndian.AppendUint64(tai64nTimestampBuf, uint64(epochOffset+now.Unix()))
	tai64nTimestampBuf = binary.BigEndian.AppendUint32(tai64nTimestampBuf, uint32(now.Nanosecond()))
	msg, _, _, err := hs.WriteMessage(nil, tai64nTimestampBuf)
	if err != nil {
		return 0, err
	}

	initiationPacket := new(bytes.Buffer)
	binary.Write(initiationPacket, binary.BigEndian, []byte{0x01, 0x00, 0x00, 0x00})
	binary.Write(initiationPacket, binary.BigEndian, uint32ToBytes(28))
	binary.Write(initiationPacket, binary.BigEndian, msg)

	macKey := blake2s.Sum256(append([]byte("mac1----"), peerPublicKey...))
	hasher, err := blake2s.New128(macKey[:]) // using macKey as the key
	if err != nil {
		return 0, err
	}
	_, err = hasher.Write(initiationPacket.Bytes())
	if err != nil {
		return 0, err
	}
	initiationPacketMAC := hasher.Sum(nil)

	// Append the MAC and 16 null bytes to the initiation packet
	binary.Write(initiationPacket, binary.BigEndian, initiationPacketMAC[:16])
	binary.Write(initiationPacket, binary.BigEndian, [16]byte{})

	conn, err := net.Dial("udp", serverAddr.String())
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	numPackets := randomInt(8, 15)
	randomPacket := make([]byte, 100)
	for i := uint64(0); i < numPackets; i++ {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			packetSize := randomInt(40, 100)
			_, err := rand.Read(randomPacket[:packetSize])
			if err != nil {
				return 0, fmt.Errorf("error generating random packet: %w", err)
			}

			_, err = conn.Write(randomPacket[:packetSize])
			if err != nil {
				return 0, fmt.Errorf("error sending random packet: %w", err)
			}

			time.Sleep(time.Duration(randomInt(20, 250)) * time.Millisecond)
		}
	}

	_, err = initiationPacket.WriteTo(conn)
	if err != nil {
		return 0, err
	}
	t0 := time.Now()

	response := make([]byte, 92)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	i, err := conn.Read(response)
	if err != nil {
		return 0, err
	}
	rtt := time.Since(t0)

	if i < 60 {
		return 0, fmt.Errorf("invalid handshake response length %d bytes", i)
	}

	// Check the response type
	if response[0] != 2 { // 2 is the message type for response
		return 0, errors.New("invalid response type")
	}

	// Extract sender and receiver index from the response
	// peer index
	_ = binary.LittleEndian.Uint32(response[4:8])
	// our index(we set it to 28)
	ourIndex := binary.LittleEndian.Uint32(response[8:12])
	if ourIndex != 28 { // Check if the response corresponds to our sender index
		return 0, errors.New("invalid sender index in response")
	}

	payload, _, _, err := hs.ReadMessage(nil, response[12:60])
	if err != nil {
		return 0, err
	}

	// Check if the payload is empty (as expected in WireGuard handshake)
	if len(payload) != 0 {
		return 0, errors.New("unexpected payload in response")
	}

	return rtt, nil
}

func NewWarpPing(ip netip.Addr, opts *statute.ScannerOptions) *WarpPing {
	return &WarpPing{
		PrivateKey:    opts.WarpPrivateKey,
		PeerPublicKey: opts.WarpPeerPublicKey,
		PresharedKey:  opts.WarpPresharedKey,
		IP:            ip,

		opts: *opts,
	}
}

var (
	_ statute.IPing       = (*WarpPing)(nil)
	_ statute.IPingResult = (*WarpPingResult)(nil)
)
