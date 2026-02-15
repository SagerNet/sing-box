package sniff

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

// BitTorrent detects if the stream is a BitTorrent connection.
// For the BitTorrent protocol specification, see https://www.bittorrent.org/beps/bep_0003.html
func BitTorrent(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var first byte
	err := binary.Read(reader, binary.BigEndian, &first)
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}

	if first != 19 {
		return os.ErrInvalid
	}

	const header = "BitTorrent protocol"
	var protocol [19]byte
	var n int
	n, err = reader.Read(protocol[:])
	if string(protocol[:n]) != header[:n] {
		return os.ErrInvalid
	}
	if err != nil {
		return E.Cause1(ErrNeedMoreData, err)
	}
	if n < 19 {
		return ErrNeedMoreData
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// UTP detects if the packet is a uTP connection packet with robust false positive rejection.
// For the uTP protocol specification, see https://www.bittorrent.org/beps/bep_0029.html
func UTP(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if len(packet) < 20 {
		return os.ErrInvalid
	}

	// Reject DHCP/BOOTP packets (RFC 2131)
	if len(packet) >= 240 {
		op := packet[0]
		htype := packet[1]
		hlen := packet[2]
		if (op == 0x01 || op == 0x02) && htype == 0x01 && hlen == 0x06 {
			if packet[236] == 0x63 && packet[237] == 0x82 && packet[238] == 0x53 && packet[239] == 0x63 {
				return os.ErrInvalid
			}
		}
	}

	// Reject modern STUN (RFC 5389) — magic cookie at offset 4-7
	if len(packet) >= 8 {
		if packet[4] == 0x21 && packet[5] == 0x12 && packet[6] == 0xA4 && packet[7] == 0x42 {
			return os.ErrInvalid
		}
	}

	// Reject classic STUN (RFC 3489) — known binding message types
	if len(packet) >= 20 {
		msgType := binary.BigEndian.Uint16(packet[0:2])
		msgLen := binary.BigEndian.Uint16(packet[2:4])
		if msgType < 0x4000 && msgLen < 1500 {
			if msgType == 0x0001 || msgType == 0x0101 || msgType == 0x0111 ||
				msgType == 0x0002 || msgType == 0x0102 || msgType == 0x0112 {
				return os.ErrInvalid
			}
		}
	}

	// Reject DTLS packets — DTLS versions at offset 5-6
	if len(packet) >= 7 {
		version := binary.BigEndian.Uint16(packet[5:7])
		if version == 0xFEFF || version == 0xFEFD || version == 0xFEFC {
			return os.ErrInvalid
		}
	}

	// Validate uTP version and type
	version := packet[0] & 0x0F
	typ := packet[0] >> 4
	if version != 1 || typ > 4 {
		return os.ErrInvalid
	}

	connectionID := binary.BigEndian.Uint16(packet[2:4])
	windowSize := binary.BigEndian.Uint32(packet[12:16])

	// Reject zero connection ID for non-SYN packets
	if connectionID == 0 && typ != 4 {
		return os.ErrInvalid
	}

	// Reject unrealistically large window sizes
	if windowSize > maxUTPWindowSize {
		return os.ErrInvalid
	}

	// Reject WireGuard handshake initiation: type=0 with 0x00 0x00 0x00 reserved
	if typ == 0 && len(packet) >= 4 {
		if packet[1] == 0x00 && packet[2] == 0x00 && packet[3] == 0x00 {
			return os.ErrInvalid
		}
	}

	// Reject VoIP/messaging protocols based on timestamp_diff patterns
	if typ == 0 || typ == 1 {
		timestampDiff := binary.BigEndian.Uint32(packet[8:12])

		zeroCount := 0
		for _, b := range packet[8:12] {
			if b == 0 {
				zeroCount++
			}
		}
		if len(packet) >= 200 && zeroCount >= 3 {
			return os.ErrInvalid
		}
		if len(packet) >= 100 && zeroCount == 4 {
			return os.ErrInvalid
		}
		if timestampDiff > 2000000000 {
			return os.ErrInvalid
		}
	}

	// Validate initial extension field (must be 0-4 per BEP 29)
	extension := packet[1]
	if extension > 4 {
		return os.ErrInvalid
	}

	// Walk extension linked list
	offset := 20
	for extension != 0 {
		if offset >= len(packet) {
			return os.ErrInvalid
		}
		nextExtension := packet[offset]
		offset++
		if nextExtension > 4 {
			return os.ErrInvalid
		}
		if offset >= len(packet) {
			return os.ErrInvalid
		}
		length := int(packet[offset])
		offset++
		extension = nextExtension
		offset += length
		if offset > len(packet) {
			return os.ErrInvalid
		}
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// UDPTracker detects if the packet is a UDP Tracker Protocol packet with deep validation.
// For the UDP Tracker Protocol specification, see https://www.bittorrent.org/beps/bep_0015.html
func UDPTracker(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if len(packet) < minSizeConnect {
		return os.ErrInvalid
	}

	// Reject DNS queries and responses
	if len(packet) >= 12 {
		flags := binary.BigEndian.Uint16(packet[2:4])
		qdcount := binary.BigEndian.Uint16(packet[4:6])
		isQuery := (flags&0x8000) == 0 && qdcount > 0 && qdcount < 100
		isResponse := (flags & 0x8000) != 0
		if isQuery || isResponse {
			opcode := (flags >> 11) & 0x0F
			if opcode <= 2 {
				return os.ErrInvalid
			}
		}
	}

	// Reject CAPWAP control packets
	if packet[0] == 0x00 && (packet[1] == 0x10 || packet[1] == 0x20 || packet[1] == 0x00) {
		if len(packet) >= 14 && packet[12] == 0x00 && packet[13] == 0x00 {
			return os.ErrInvalid
		}
	}

	// Reject DTLS packets
	if len(packet) >= 3 {
		contentType := packet[0]
		dtlsVersion := binary.BigEndian.Uint16(packet[1:3])
		if (contentType >= 0x14 && contentType <= 0x17) &&
			(dtlsVersion == 0xFEFF || dtlsVersion == 0xFEFD || dtlsVersion == 0xFEFC) {
			return os.ErrInvalid
		}
	}

	// Reject AFS RX protocol packets
	if len(packet) >= 24 {
		epoch := binary.BigEndian.Uint32(packet[0:4])
		callNum := binary.BigEndian.Uint32(packet[8:12])
		seq := binary.BigEndian.Uint32(packet[12:16])
		serial := binary.BigEndian.Uint32(packet[16:20])
		packetType := packet[20]
		if epoch >= 0x50000000 &&
			callNum <= 100 &&
			seq <= 1000 &&
			serial <= 1000 &&
			packetType >= 1 && packetType <= 13 {
			return os.ErrInvalid
		}
	}

	// 1. Connect (Magic Number Check)
	if len(packet) >= minSizeConnect && len(packet) < minSizeScrape {
		if binary.BigEndian.Uint64(packet[:8]) == trackerProtocolID &&
			binary.BigEndian.Uint32(packet[8:12]) == actionConnect {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	// 2. Announce (Action + PeerID Check)
	if len(packet) >= minSizeAnnounce {
		action := binary.BigEndian.Uint32(packet[8:12])
		if action == actionAnnounce {
			connectionID := binary.BigEndian.Uint64(packet[:8])
			if connectionID == 0 || connectionID == trackerProtocolID {
				return os.ErrInvalid
			}
			if countTrailingZeroBytes(packet[:8]) > 3 {
				return os.ErrInvalid
			}

			// Check PeerID at offset 36
			peerID := packet[36:40]
			for _, prefix := range peerIDPrefixes {
				if bytes.HasPrefix(peerID, prefix) {
					metadata.Protocol = C.ProtocolBitTorrent
					return nil
				}
			}

			// Without known peer ID prefix, validate info_hash
			infoHash := packet[16:36]
			allZero := true
			allFF := true
			for _, b := range infoHash {
				if b != 0 {
					allZero = false
				}
				if b != 0xFF {
					allFF = false
				}
				if !allZero && !allFF {
					break
				}
			}
			if allZero || allFF {
				return os.ErrInvalid
			}

			// Reject excessive trailing zeros in peer ID
			if countTrailingZeroBytes(packet[36:56]) > 3 {
				return os.ErrInvalid
			}

			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	// 3. Scrape
	if len(packet) >= minSizeScrape {
		action := binary.BigEndian.Uint32(packet[8:12])
		if action == actionScrape {
			connectionID := binary.BigEndian.Uint64(packet[:8])
			if connectionID == 0 || connectionID == trackerProtocolID {
				return os.ErrInvalid
			}
			if countTrailingZeroBytes(packet[:8]) > 3 {
				return os.ErrInvalid
			}
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	return os.ErrInvalid
}

// BitTorrentDHTPacket detects BitTorrent DHT packets (bencode dictionary + node validation).
func BitTorrentDHTPacket(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if checkBencodeDHT(packet) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}
	return os.ErrInvalid
}

// BitTorrentLSD detects Local Service Discovery (BEP 26) multicast traffic.
func BitTorrentLSD(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	// Check destination address and port
	if metadata.Destination.Port == 6771 && metadata.Destination.Addr.IsValid() {
		dest := metadata.Destination.Addr
		if dest == netip.AddrFrom4([4]byte{239, 192, 152, 143}) {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
		lsdIPv6, _ := netip.AddrFromSlice([]byte{0xff, 0x15, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xef, 0xc0, 0x98, 0x8f})
		if dest == lsdIPv6 {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	// Check payload for LSD message patterns
	if bytes.Contains(packet, []byte("BT-SEARCH * HTTP/1.1")) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}
	if bytes.Contains(packet, []byte("Host: 239.192.152.143:6771")) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}
	if bytes.Contains(packet, []byte("Infohash: ")) &&
		bytes.Contains(packet, []byte("Port: ")) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	return os.ErrInvalid
}

// BitTorrentSignaturePacket detects BitTorrent UDP packets by signature matching.
func BitTorrentSignaturePacket(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if checkSignatures(packet) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}
	return os.ErrInvalid
}

// BitTorrentMSE detects Message Stream Encryption (MSE/PE) handshakes in TCP streams.
func BitTorrentMSE(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	// MSE minimum: 96-byte DH key + VC (8 bytes) + crypto field (4 bytes) = 108
	var buf [628]byte
	n, _ := io.ReadFull(reader, buf[:108])
	if n < 108 {
		return os.ErrInvalid
	}

	// Read more data if available (up to 628 bytes for padding scan)
	if n == 108 {
		extra, _ := reader.Read(buf[108:])
		n += extra
	}

	payload := buf[:n]

	// Phase 1: Distinct-byte pre-check
	var seen [256]bool
	distinct := 0
	for _, b := range payload[:96] {
		if !seen[b] {
			seen[b] = true
			distinct++
		}
	}
	if distinct < 92 {
		return os.ErrInvalid
	}

	// Phase 2: Shannon entropy check — DH public keys should have high entropy (> 6.5)
	if shannonEntropy(payload[0:96]) <= 6.5 {
		return os.ErrInvalid
	}

	// Phase 3: VC scan — look for 8 consecutive zero bytes
	searchEnd := n
	if searchEnd > 628 {
		searchEnd = 628
	}

	vcPosition := -1
	zeroRun := 0
	for i := 96; i < searchEnd; i++ {
		if payload[i] == 0 {
			zeroRun++
			if zeroRun == 8 {
				vcPosition = i - 7
				break
			}
		} else {
			zeroRun = 0
		}
	}
	if vcPosition < 0 {
		return os.ErrInvalid
	}

	// Phase 4: crypto field check — valid values: 0x01 (plaintext) or 0x02 (RC4)
	if n < vcPosition+12 {
		return os.ErrInvalid
	}
	cryptoBytes := binary.BigEndian.Uint32(payload[vcPosition+8 : vcPosition+12])
	if cryptoBytes > 0 && cryptoBytes <= 0x03 {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	return os.ErrInvalid
}

// BitTorrentMessage detects BitTorrent TCP message structure (length-prefixed messages).
func BitTorrentMessage(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var header [53]byte
	n, _ := io.ReadFull(reader, header[:5])
	if n < 5 {
		return os.ErrInvalid
	}

	// Read more if we need it (up to 53 bytes for piece SSH rejection)
	total := 5
	if n == 5 {
		extra, _ := reader.Read(header[5:])
		total += extra
	}

	payload := header[:total]
	if !checkBitTorrentMessage(payload) {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// BitTorrentFAST detects FAST Extension messages (BEP 6).
func BitTorrentFAST(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var header [5]byte
	n, _ := io.ReadFull(reader, header[:])
	if n < 5 {
		return os.ErrInvalid
	}

	if !checkFASTExtension(header[:]) {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// BitTorrentExtended detects BitTorrent Extension Protocol messages (BEP 10).
func BitTorrentExtended(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var header [7]byte
	n, _ := io.ReadFull(reader, header[:])
	if n < 7 {
		return os.ErrInvalid
	}

	if !checkExtendedMessage(header[:]) {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// BitTorrentHTTP detects HTTP-based BitTorrent protocols (WebSeed, Bitcomet, User-Agent).
func BitTorrentHTTP(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var buf [512]byte
	n, _ := io.ReadAtLeast(reader, buf[:], 16)
	if n < 16 {
		return os.ErrInvalid
	}

	if !checkHTTPBitTorrent(buf[:n]) {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// BitTorrentSignature detects BitTorrent TCP streams by signature matching.
func BitTorrentSignature(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	var buf [512]byte
	n, _ := io.ReadAtLeast(reader, buf[:], 4)
	if n < 4 {
		return os.ErrInvalid
	}

	if !checkSignatures(buf[:n]) {
		return os.ErrInvalid
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// --- Helper functions ---

// shannonEntropy calculates the Shannon entropy of data.
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var freq [256]int
	for _, b := range data {
		freq[b]++
	}
	entropy := 0.0
	total := float64(len(data))
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / total
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// countTrailingZeroBytes counts trailing zero bytes in a byte slice.
func countTrailingZeroBytes(data []byte) int {
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == 0 {
			count++
		} else {
			break
		}
	}
	return count
}

// parseBencodeLength parses a decimal length from bencode with overflow protection.
func parseBencodeLength(data []byte) int {
	const maxLen = 1 << 20 // 1MB cap
	n := 0
	for _, ch := range data {
		if ch < '0' || ch > '9' {
			continue
		}
		n = n*10 + int(ch-'0')
		if n > maxLen {
			return 0
		}
	}
	return n
}

// checkDHTNodes validates DHT node list binary structure.
// IPv4 nodes: 26 bytes per node (20-byte ID + 4-byte IP + 2-byte port)
// IPv6 nodes: 38 bytes per node (20-byte ID + 16-byte IP + 2-byte port)
func checkDHTNodes(payload []byte) bool {
	// Check for IPv4 nodes list: "6:nodes<len>:<data>"
	nodesIdx := bytes.Index(payload, []byte("6:nodes"))
	if nodesIdx != -1 && nodesIdx+7 < len(payload) {
		offset := nodesIdx + 7
		colonIdx := bytes.IndexByte(payload[offset:], ':')
		if colonIdx != -1 && colonIdx > 0 && colonIdx < 10 {
			nodeDataLen := parseBencodeLength(payload[offset : offset+colonIdx])
			if nodeDataLen >= 26 && nodeDataLen%26 == 0 {
				return true
			}
		}
	}

	// Check for IPv6 nodes list: "7:nodes6<len>:<data>"
	nodes6Idx := bytes.Index(payload, []byte("7:nodes6"))
	if nodes6Idx != -1 && nodes6Idx+8 < len(payload) {
		offset := nodes6Idx + 8
		colonIdx := bytes.IndexByte(payload[offset:], ':')
		if colonIdx != -1 && colonIdx > 0 && colonIdx < 10 {
			nodeDataLen := parseBencodeLength(payload[offset : offset+colonIdx])
			if nodeDataLen >= 38 && nodeDataLen%38 == 0 {
				return true
			}
		}
	}

	return false
}

// checkBencodeDHT looks for structural Bencode dictionary patterns with DHT validation.
func checkBencodeDHT(payload []byte) bool {
	if len(payload) < 8 {
		return false
	}
	if payload[0] != 'd' || payload[len(payload)-1] != 'e' {
		return false
	}

	// Check for Suricata-specific prefixes
	if bytes.HasPrefix(payload, []byte("d1:ad")) ||
		bytes.HasPrefix(payload, []byte("d1:rd")) ||
		bytes.HasPrefix(payload, []byte("d2:ip")) ||
		bytes.HasPrefix(payload, []byte("d1:el")) {
		return true
	}

	// Must contain query/response/error type
	hasQuery := bytes.Contains(payload, []byte("1:y1:q"))
	hasResponse := bytes.Contains(payload, []byte("1:y1:r"))
	hasError := bytes.Contains(payload, []byte("1:y1:e"))
	if !hasQuery && !hasResponse && !hasError {
		return false
	}

	hasDHTMethod := bytes.Contains(payload, []byte("4:ping")) ||
		bytes.Contains(payload, []byte("9:find_node")) ||
		bytes.Contains(payload, []byte("9:get_peers")) ||
		bytes.Contains(payload, []byte("13:announce_peer")) ||
		bytes.Contains(payload, []byte("3:get")) ||
		bytes.Contains(payload, []byte("3:put"))

	// Check for transaction ID AND (DHT method OR DHT-specific fields)
	if bytes.Contains(payload, []byte("1:t")) {
		if hasQuery {
			return hasDHTMethod
		}
		return hasDHTMethod ||
			checkDHTNodes(payload) ||
			bytes.Contains(payload, []byte("6:values")) ||
			bytes.Contains(payload, []byte("5:token"))
	}

	if checkDHTNodes(payload) {
		return true
	}

	return false
}

// checkSignatures searches for BitTorrent signature patterns in payload.
func checkSignatures(payload []byte) bool {
	// Fast-path: most common signatures
	if bytes.Contains(payload, []byte("BitTorrent protocol")) {
		return true
	}

	// DHT queries/responses fast-path
	if len(payload) >= 13 && payload[0] == 'd' && payload[1] == '1' && payload[2] == ':' {
		if payload[3] == 'a' || payload[3] == 'r' {
			if payload[4] == 'd' && payload[5] == '2' && payload[6] == ':' {
				return true
			}
		}
	}

	// Check remaining signatures (skip indices 0 and 1, already checked)
	for _, sig := range btSignatures[2:] {
		if len(sig) > len(payload) {
			continue
		}
		if bytes.Contains(payload, sig) {
			return true
		}
	}
	return false
}

// checkExtendedMessage detects BitTorrent Extension Protocol messages (BEP 10).
func checkExtendedMessage(payload []byte) bool {
	if len(payload) < 7 {
		return false
	}
	// Message ID 20 (0x14) at offset 4
	if payload[4] == 0x14 {
		if len(payload) > 6 && payload[6] == 'd' {
			return true
		}
		return true
	}
	return false
}

// checkFASTExtension detects FAST Extension messages (BEP 6).
func checkFASTExtension(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}
	msgID := payload[4]
	if msgID >= 0x0D && msgID <= 0x11 {
		msgLen := binary.BigEndian.Uint32(payload[0:4])
		switch msgID {
		case 0x0D, 0x11: // Suggest Piece, Allowed Fast — 5 bytes
			return msgLen == 5
		case 0x0E, 0x0F: // Have All, Have None — 1 byte
			return msgLen == 1
		case 0x10: // Reject Request — 13 bytes
			return msgLen == 13
		}
		return true
	}
	return false
}

// checkHTTPBitTorrent detects HTTP-based BitTorrent protocols.
func checkHTTPBitTorrent(payload []byte) bool {
	if len(payload) < 16 {
		return false
	}
	if !bytes.HasPrefix(payload, []byte("GET ")) {
		return false
	}

	if bytes.Contains(payload, []byte("/webseed?info_hash=")) {
		return true
	}
	if bytes.Contains(payload, []byte("/data?fid=")) && bytes.Contains(payload, []byte("&size=")) {
		return true
	}
	if bytes.Contains(payload, []byte("User-Agent: Azureus")) ||
		bytes.Contains(payload, []byte("User-Agent: BitTorrent")) ||
		bytes.Contains(payload, []byte("User-Agent: BTWebClient")) ||
		bytes.Contains(payload, []byte("User-Agent: FlashGet")) {
		return true
	}
	// Shareaza with Gnutella exclusion
	if bytes.Contains(payload, []byte("User-Agent: Shareaza")) {
		if bytes.Contains(payload, []byte("GNUTELLA/")) {
			return false
		}
		return true
	}

	return false
}

// checkBitTorrentMessage detects BitTorrent TCP messages by structure.
func checkBitTorrentMessage(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}

	msgID := payload[4]

	// Reject SSH ranges (50+)
	if msgID >= 50 {
		return false
	}

	// SSH transport layer (21-49): only accept BT v2 hash messages (21-23)
	if msgID >= 21 && msgID <= 49 {
		if msgID > 23 {
			return false
		}
	}

	msgLen := binary.BigEndian.Uint32(payload[0:4])
	if msgLen == 0 || msgLen > 262144 {
		return false
	}

	expectedLen := int(msgLen) + 4
	if expectedLen > len(payload)*10 {
		return false
	}

	switch msgID {
	case 0x00, 0x01, 0x02, 0x03: // Choke, Unchoke, Interested, Not Interested
		return msgLen == 1

	case 0x04: // Have
		return msgLen == 5

	case 0x05: // Bitfield
		if msgLen <= 1 || msgLen > 65536 {
			return false
		}
		// Reject short bitfields that look like protocol messages (MSDO, SSH)
		if msgLen >= 8 && msgLen <= 12 {
			if len(payload) >= 9 {
				data := payload[5:]
				ffCount := 0
				zeroCount := 0
				for _, b := range data {
					if b == 0xFF {
						ffCount++
					} else if b == 0x00 {
						zeroCount++
					}
				}
				if (ffCount + zeroCount) >= len(data)*6/10 {
					return false
				}
			}
		}
		// Reject encrypted SSH packets (high unique byte count)
		if len(payload) >= 20 && msgLen > 40 {
			sample := payload[5:21]
			var seen [256]bool
			uniqueCount := 0
			repeatedCount := 0
			prevByte := sample[0]
			for _, b := range sample {
				if !seen[b] {
					seen[b] = true
					uniqueCount++
				}
				if b == prevByte {
					repeatedCount++
				}
				prevByte = b
			}
			if uniqueCount >= 13 && repeatedCount <= 4 {
				return false
			}
		}
		return true

	case 0x06: // Request
		return msgLen == 13

	case 0x07: // Piece
		if msgLen <= 9 || msgLen > 16393 {
			return false
		}
		// Reject SSH key exchange (high ASCII + commas/hyphens)
		if len(payload) >= 50 {
			sampleStart := 13
			if sampleStart+40 <= len(payload) {
				sample := payload[sampleStart : sampleStart+40]
				printableCount := 0
				commaCount := 0
				for _, b := range sample {
					if b >= 0x20 && b <= 0x7E {
						printableCount++
						if b == ',' || b == '-' {
							commaCount++
						}
					}
				}
				if printableCount >= 30 && commaCount >= 3 {
					return false
				}
			}
		}
		return true

	case 0x08: // Cancel
		return msgLen == 13

	case 0x09: // Port (DHT)
		return msgLen == 3

	case 0x0D: // Suggest Piece (BEP 6)
		return msgLen == 5

	case 0x0E, 0x0F: // Have All, Have None (BEP 6)
		return msgLen == 1

	case 0x10: // Reject Request (BEP 6)
		return msgLen == 13

	case 0x11: // Allowed Fast (BEP 6)
		return msgLen == 5

	case 0x14: // Extended (BEP 10)
		if msgLen <= 1 {
			return false
		}
		if len(payload) >= 6 {
			extID := payload[5]
			if extID == 0 {
				if len(payload) > 6 && payload[6] == 'd' {
					return true
				}
			} else {
				return msgLen > 2 && msgLen < 131072
			}
		}
		return msgLen > 1 && msgLen < 131072

	case 0x15, 0x16, 0x17: // Hash request, Hashes, Hash reject (BEP 52)
		return msgLen > 1 && msgLen < 131072

	default:
		return false
	}
}
