package sniff

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	trackerConnectFlag    = 0
	trackerProtocolID     = 0x41727101980
	trackerConnectMinSize = 16
	trackerActionAnnounce = 1
	trackerActionScrape   = 2
	minSizeAnnounce       = 98
	minSizeScrape         = 36
)

// BitTorrent signature patterns for detection
var btSignatures = [][]byte{
	// Standard protocol headers
	[]byte("\x13BitTorrent protocol"),
	[]byte("BitTorrent protocol"),

	// Extension Protocol (BEP 10)
	[]byte("ut_metadata"),
	[]byte("ut_pex"),
	[]byte("12:ut_holepunch"),
	[]byte("11:upload_only"),
	[]byte("13:metadata_size"),
	[]byte("8:msg_type"),

	// PEX (Peer Exchange) Keys
	[]byte("5:added"),
	[]byte("7:added.f"),
	[]byte("7:dropped"),
	[]byte("6:added6"),
	[]byte("8:added6.f"),

	// DHT Bencode Keys
	[]byte("d1:ad2:id20:"),
	[]byte("d1:rd2:id20:"),
	[]byte("1:y1:q"),
	[]byte("1:y1:r"),
	[]byte("1:y1:e"),
	[]byte("find_node"),
	[]byte("9:get_peers"),
	[]byte("13:announce_peer"),
	[]byte("5:token"),
	[]byte("6:values"),

	// Magnet links and tracker URLs
	[]byte("magnet:?xt=urn:btih:"),
	[]byte("info_hash"),
	[]byte("peer_id="),
	[]byte("uploaded="),
	[]byte("downloaded="),

	// LSD (Local Service Discovery)
	[]byte("BT-SEARCH * HTTP/1.1"),
	[]byte("Host: 239.192.152.143:6771"),

	// HTTP-based BitTorrent
	[]byte("GET /webseed?info_hash="),
	[]byte("User-Agent: Azureus"),
	[]byte("User-Agent: BitTorrent"),
	[]byte("User-Agent: BTWebClient"),
}

// checkSignatures searches for BitTorrent signature patterns in payload
func checkSignatures(payload []byte) bool {
	for _, sig := range btSignatures {
		if bytes.Contains(payload, sig) {
			return true
		}
	}
	return false
}

// shannonEntropy calculates data randomness/entropy
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	freq := make([]int, 256)
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

// checkMSEEncryption detects Message Stream Encryption (MSE/PE) handshake
func checkMSEEncryption(payload []byte) bool {
	// MSE handshake: 96-byte DH key + optional padding + 8-byte VC (zero bytes)
	if len(payload) < 104 {
		return false
	}

	// Strategy 1: Look for Verification Constant (8 consecutive zero bytes)
	searchEnd := 628
	if len(payload) < searchEnd {
		searchEnd = len(payload)
	}

	for i := 96; i <= searchEnd-8 && i < len(payload)-8; i++ {
		isVC := true
		for j := 0; j < 8; j++ {
			if payload[i+j] != 0x00 {
				isVC = false
				break
			}
		}
		if isVC {
			return true
		}
	}

	// Strategy 2: Check if first 96 bytes have high entropy (DH public key)
	if len(payload) >= 96 {
		entropy := shannonEntropy(payload[0:96])
		if entropy > 7.0 {
			return true
		}
	}

	return false
}

// checkBencodeDHT validates DHT Bencode structure
func checkBencodeDHT(payload []byte) bool {
	if len(payload) < 8 {
		return false
	}
	// Must start with 'd' and end with 'e'
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
	hasType := bytes.Contains(payload, []byte("1:y1:q")) ||
		bytes.Contains(payload, []byte("1:y1:r")) ||
		bytes.Contains(payload, []byte("1:y1:e"))
	if !hasType {
		return false
	}

	// Check for transaction ID or DHT-specific fields
	return bytes.Contains(payload, []byte("1:t")) ||
		bytes.Contains(payload, []byte("6:values")) ||
		bytes.Contains(payload, []byte("5:token"))
}

// checkExtendedMessage detects BitTorrent Extension Protocol messages (BEP 10)
func checkExtendedMessage(payload []byte) bool {
	if len(payload) < 7 {
		return false
	}
	// Check for message ID 20 (0x14) at offset 4
	if payload[4] == 0x14 {
		// Extended handshake starts with 'd'
		if len(payload) > 6 && payload[6] == 'd' {
			return true
		}
		return true
	}
	return false
}

// checkFASTExtension detects FAST Extension messages (BEP 6)
func checkFASTExtension(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}

	msgID := payload[4]
	// FAST extension message IDs: 13-17 (0x0D-0x11)
	if msgID >= 0x0D && msgID <= 0x11 {
		msgLen := binary.BigEndian.Uint32(payload[0:4])
		switch msgID {
		case 0x0D, 0x11: // Suggest Piece, Allowed Fast
			return msgLen == 5
		case 0x0E, 0x0F: // Have All, Have None
			return msgLen == 1
		case 0x10: // Reject Request
			return msgLen == 13
		}
		return true
	}
	return false
}

// checkHTTPBitTorrent detects HTTP-based BitTorrent protocols
func checkHTTPBitTorrent(payload []byte) bool {
	if len(payload) < 16 || !bytes.HasPrefix(payload, []byte("GET ")) {
		return false
	}

	// WebSeed Protocol (BEP 19)
	if bytes.Contains(payload, []byte("/webseed?info_hash=")) {
		return true
	}

	// Bitcomet Persistent Seed
	if bytes.Contains(payload, []byte("/data?fid=")) && bytes.Contains(payload, []byte("&size=")) {
		return true
	}

	// User-Agent Detection
	return bytes.Contains(payload, []byte("User-Agent: Azureus")) ||
		bytes.Contains(payload, []byte("User-Agent: BitTorrent")) ||
		bytes.Contains(payload, []byte("User-Agent: BTWebClient")) ||
		bytes.Contains(payload, []byte("User-Agent: Shareaza")) ||
		bytes.Contains(payload, []byte("User-Agent: FlashGet"))
}

// BitTorrent detects if the stream is a BitTorrent connection.
// For the BitTorrent protocol specification, see https://www.bittorrent.org/beps/bep_0003.html
// This function now includes enhanced detection for:
// - Standard BitTorrent handshake
// - Extended Protocol messages (BEP 10)
// - FAST Extension messages (BEP 6)
// - MSE/PE encryption
// - HTTP-based BitTorrent (WebSeed, etc.)
// - Signature-based detection
func BitTorrent(_ context.Context, metadata *adapter.InboundContext, reader io.Reader) error {
	// Read initial data for detection (up to 512 bytes for comprehensive analysis)
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if n == 0 {
		if err != nil {
			return E.Cause1(ErrNeedMoreData, err)
		}
		return ErrNeedMoreData
	}

	payload := buffer[:n]

	// 1. Check for Extended Protocol messages (BEP 10) - very specific
	if checkExtendedMessage(payload) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 2. Check for FAST Extension messages (BEP 6) - very specific
	if checkFASTExtension(payload) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 3. Check for HTTP-based BitTorrent protocols
	if checkHTTPBitTorrent(payload) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 4. Check for MSE/PE encryption - critical for encrypted traffic
	if checkMSEEncryption(payload) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 5. Signature-based detection - catches common patterns
	if checkSignatures(payload) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 6. Standard BitTorrent handshake detection
	// Format: <pstrlen><pstr><reserved><info_hash><peer_id>
	// where pstrlen=19 and pstr="BitTorrent protocol"
	if payload[0] == 19 {
		// Need at least 20 bytes to validate: 1 (length) + 19 (protocol string)
		if n < 20 {
			// Check if what we have so far matches the beginning of "BitTorrent protocol"
			const header = "BitTorrent protocol"
			if n > 1 {
				// Check if the partial data matches the protocol header
				partialHeader := header[:n-1]
				if string(payload[1:n]) != partialHeader {
					// Doesn't match even partially - invalid
					return os.ErrInvalid
				}
			}
			// Partial match or just the length byte - need more data
			return ErrNeedMoreData
		}

		const header = "BitTorrent protocol"
		if string(payload[1:20]) == header {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
		// If the first byte is 19 but the protocol string doesn't match, it's invalid
		return os.ErrInvalid
	}

	return os.ErrInvalid
}

// UTP detects if the packet is a uTP connection packet.
// For the uTP protocol specification, see
//  1. https://www.bittorrent.org/beps/bep_0029.html
//  2. https://github.com/bittorrent/libutp/blob/2b364cbb0650bdab64a5de2abb4518f9f228ec44/utp_internal.cpp#L112
// Enhanced with robust extension chain validation from BitTorrentBlocker
func UTP(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	// A valid uTP packet must be at least 20 bytes long.
	if len(packet) < 20 {
		return os.ErrInvalid
	}

	version := packet[0] & 0x0F
	ty := packet[0] >> 4
	if version != 1 || ty > 4 {
		return os.ErrInvalid
	}

	// Validate the extension chain with improved bounds checking
	extension := packet[1]

	// Validate initial extension type (known uTP extensions are 0-4, or 0 for none)
	if extension > 0x04 {
		return os.ErrInvalid
	}

	offset := 20

	// Walk through extension linked list with robust validation
	for extension != 0 {
		// Check if we can read the next extension byte
		if offset >= len(packet) {
			return os.ErrInvalid
		}
		nextExtension := packet[offset]
		offset++

		// Validate extension type (known uTP extensions are 0-4)
		if nextExtension > 0x04 {
			return os.ErrInvalid
		}

		// Check if we can read the length byte
		if offset >= len(packet) {
			return os.ErrInvalid
		}
		length := int(packet[offset])
		offset++

		// Move to next extension
		extension = nextExtension
		offset += length

		// Validate that offset doesn't exceed packet length
		if offset > len(packet) {
			return os.ErrInvalid
		}
	}

	metadata.Protocol = C.ProtocolBitTorrent
	return nil
}

// UDPTracker detects if the packet is a UDP Tracker Protocol packet.
// For the UDP Tracker Protocol specification, see https://www.bittorrent.org/beps/bep_0015.html
// Enhanced to detect Connect, Announce, and Scrape actions
func UDPTracker(_ context.Context, metadata *adapter.InboundContext, packet []byte) error {
	if len(packet) < trackerConnectMinSize {
		return os.ErrInvalid
	}

	// Check for DHT Bencode structure first (can be in UDP packets)
	if checkBencodeDHT(packet) {
		metadata.Protocol = C.ProtocolBitTorrent
		return nil
	}

	// 1. Connect request (16 bytes): protocol_id(8) + action(4) + transaction_id(4)
	if len(packet) >= 16 && len(packet) < minSizeScrape {
		if binary.BigEndian.Uint64(packet[:8]) == trackerProtocolID &&
			binary.BigEndian.Uint32(packet[8:12]) == trackerConnectFlag {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	// 2. Announce request (98 bytes minimum)
	if len(packet) >= minSizeAnnounce {
		action := binary.BigEndian.Uint32(packet[8:12])
		if action == trackerActionAnnounce {
			// Valid announce packet structure
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	// 3. Scrape request (36 bytes minimum for single info_hash)
	if len(packet) >= minSizeScrape {
		action := binary.BigEndian.Uint32(packet[8:12])
		if action == trackerActionScrape {
			metadata.Protocol = C.ProtocolBitTorrent
			return nil
		}
	}

	return os.ErrInvalid
}
