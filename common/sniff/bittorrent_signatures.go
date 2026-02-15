package sniff

// Protocol constants for BitTorrent UDP tracker (BEP 15)
const (
	trackerProtocolID = 0x41727101980

	actionConnect  = 0
	actionAnnounce = 1
	actionScrape   = 2

	minSizeConnect  = 16
	minSizeScrape   = 36
	minSizeAnnounce = 98

	maxUTPWindowSize = 100 * 1024 * 1024 // 100MB — real BT uTP typically uses 1-10MB
)

// btSignatures contains BitTorrent byte patterns from nDPI, libtorrent, Suricata, and UDPGuard.
// Index 0 and 1 are checked as a fast path before iterating the rest.
var btSignatures = [][]byte{
	// 1. Standard headers (fast-path: checked first)
	[]byte("\x13BitTorrent protocol"),
	[]byte("BitTorrent protocol"),

	// 2. Libtorrent specific
	[]byte("1:v4:LT"),
	[]byte("-LT20"),
	[]byte("-LT12"),

	// 3. PEX (Peer Exchange) Keys
	[]byte("ut_pex"),
	[]byte("5:added"),
	[]byte("7:added.f"),
	[]byte("7:dropped"),
	[]byte("6:added6"),
	[]byte("8:added6.f"),
	[]byte("8:dropped6"),

	// 4. Extension Protocol (BEP 10)
	[]byte("ut_metadata"),
	[]byte("12:ut_holepunch"),
	[]byte("11:upload_only"),
	[]byte("10:share_mode"),
	[]byte("9:lt_donthave"),
	[]byte("11:LT_metadata"),
	[]byte("13:metadata_size"),

	// 5. Text / HTTP Trackers
	[]byte("magnet:?xt=urn:btih:"),
	[]byte("magnet:?xt=urn:btmh:"),
	[]byte("udp://tracker."),
	[]byte("announce.php?passkey="),
	[]byte("supportcrypto="),
	[]byte("requirecrypto="),
	[]byte("cryptoport="),

	// 6. DHT Bencode Keys
	[]byte("d1:ad2:id20:"),
	[]byte("d1:rd2:id20:"),
	[]byte("d1:el"),
	[]byte("4:ping"),
	[]byte("9:find_node"),
	[]byte("9:get_peers"),
	[]byte("13:announce_peer"),

	// 7. LSD (Local Service Discovery)
	[]byte("BT-SEARCH * HTTP/1.1"),
	[]byte("Host: 239.192.152.143:6771"),
	[]byte("Infohash: "),

	// 8. BitTorrent v2
	[]byte("12:piece layers"),
	[]byte("9:file tree"),
	[]byte("12:pieces root"),

	// 9. HTTP-based BitTorrent
	[]byte("GET /webseed?info_hash="),
	[]byte("GET /data?fid="),
	[]byte("User-Agent: Azureus"),
	[]byte("User-Agent: BitTorrent"),
	[]byte("User-Agent: BTWebClient"),
	[]byte("User-Agent: FlashGet"),
}

// peerIDPrefixes contains known BitTorrent client Peer ID prefixes.
var peerIDPrefixes = [][]byte{
	// Azureus-style: -XX####-
	[]byte("-qB"), // qBittorrent
	[]byte("-TR"), // Transmission
	[]byte("-UT"), // µTorrent
	[]byte("-LT"), // libtorrent (rTorrent, Deluge)
	[]byte("-DE"), // Deluge
	[]byte("-BM"), // BitComet
	[]byte("-AZ"), // Azureus/Vuze
	[]byte("-lt"), // libTorrent (lowercase)
	[]byte("-KT"), // KTorrent
	[]byte("-FW"), // FrostWire
	[]byte("-XL"), // Xunlei (Thunder)
	[]byte("-SD"), // Thunder (alternative)
	[]byte("-UM"), // µTorrent Mac
	[]byte("-KG"), // KGet
	[]byte("-BB"), // BitBuddy
	[]byte("-BC"), // BitComet (alternative)
	[]byte("-BR"), // BitRocket
	[]byte("-BS"), // BTSlave
	[]byte("-BX"), // Bittorrent X
	[]byte("-CD"), // Enhanced CTorrent
	[]byte("-CT"), // CTorrent
	[]byte("-DP"), // Propagate Data Client
	[]byte("-EB"), // EBit
	[]byte("-ES"), // Electric Sheep
	[]byte("-FT"), // FoxTorrent
	[]byte("-FX"), // Freebox BitTorrent
	[]byte("-GS"), // GSTorrent
	[]byte("-HL"), // Halite
	[]byte("-HN"), // Hydranode
	[]byte("-LH"), // LH-ABC
	[]byte("-LP"), // Lphant
	[]byte("-LW"), // LimeWire
	[]byte("-MO"), // MonoTorrent
	[]byte("-MP"), // MooPolice
	[]byte("-MR"), // Miro
	[]byte("-MT"), // MoonlightTorrent
	[]byte("-NX"), // Net Transport
	[]byte("-PD"), // Pando
	[]byte("-QD"), // QQDownload
	[]byte("-QT"), // Qt 4 Torrent
	[]byte("-RT"), // Retriever
	[]byte("-SB"), // Swiftbit
	[]byte("-SS"), // SwarmScope
	[]byte("-ST"), // SymTorrent
	[]byte("-TN"), // TorrentDotNET
	[]byte("-TT"), // TuoTu
	[]byte("-UL"), // uLeecher
	[]byte("-WD"), // Web Downloader
	[]byte("-WY"), // FireTorrent
	[]byte("-XT"), // XanTorrent
	[]byte("-XX"), // Xtorrent
	[]byte("-ZT"), // ZipTorrent
	[]byte("-FG"), // FlashGet

	// Non-Azureus style
	[]byte("M4-"),   // Mainline (official BitTorrent)
	[]byte("T0"),    // BitTornado
	[]byte("OP"),    // Opera
	[]byte("XBT"),   // XBT Client
	[]byte("exbc"),  // BitComet (non-Azureus)
	[]byte("FUTB"),  // FuTorrent
	[]byte("Plus"),  // Plus! v2
	[]byte("turbo"), // Turbo BT
	[]byte("btpd"),  // BT Protocol Daemon
}
