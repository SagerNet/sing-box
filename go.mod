module github.com/sagernet/sing-box

go 1.20

require (
	berty.tech/go-libtor v1.0.385
	github.com/caddyserver/certmagic v0.19.2
	github.com/cloudflare/circl v1.3.3
	github.com/cretz/bine v0.2.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-chi/chi/v5 v5.0.10
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/render v1.0.3
	github.com/gofrs/uuid/v5 v5.0.0
	github.com/insomniacslk/dhcp v0.0.0-20230908212754-65c27093e38a
	github.com/libdns/alidns v1.0.3
	github.com/libdns/cloudflare v0.1.0
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mholt/acmez v1.2.0
	github.com/miekg/dns v1.1.56
	github.com/ooni/go-libtor v1.1.8
	github.com/oschwald/maxminddb-golang v1.12.0
	github.com/sagernet/bbolt v0.0.0-20231008142710-b2d6e2f20458
	github.com/sagernet/cloudflare-tls v0.0.0-20230829051644-4a68352d0c4a
	github.com/sagernet/gomobile v0.0.0-20230915142329-c6740b6d2950
	github.com/sagernet/gvisor v0.0.0-20230930141345-5fef6f2e17ab
	github.com/sagernet/quic-go v0.0.0-20231008035953-32727fef9460
	github.com/sagernet/reality v0.0.0-20230406110435-ee17307e7691
	github.com/sagernet/sing v0.2.14-0.20231008040725-e690cb9a7ad2
	github.com/sagernet/sing-dns v0.1.10
	github.com/sagernet/sing-mux v0.1.3
	github.com/sagernet/sing-quic v0.1.3-0.20231008043106-a107947d1ed5
	github.com/sagernet/sing-shadowsocks v0.2.5
	github.com/sagernet/sing-shadowsocks2 v0.1.4
	github.com/sagernet/sing-shadowtls v0.1.4
	github.com/sagernet/sing-tun v0.1.16-0.20231006112722-19cc8b9e81aa
	github.com/sagernet/sing-vmess v0.1.8
	github.com/sagernet/smux v0.0.0-20230312102458-337ec2a5af37
	github.com/sagernet/tfo-go v0.0.0-20230816093905-5a5c285d44a6
	github.com/sagernet/utls v0.0.0-20230309024959-6732c2ab36f2
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e
	github.com/sagernet/wireguard-go v0.0.0-20230807125731-5d4a7ef2dc5f
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.26.0
	go4.org/netipx v0.0.0-20230824141953-6213f710f925
	golang.org/x/crypto v0.14.0
	golang.org/x/exp v0.0.0-20231005195138-3e424a577f31
	golang.org/x/net v0.16.0
	golang.org/x/sys v0.13.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6
	google.golang.org/grpc v1.58.2
	google.golang.org/protobuf v1.31.0
	howett.net/plist v1.0.0
)

//replace github.com/sagernet/sing => ../sing

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.3.4 // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/scjalliance/comshim v0.0.0-20230315213746-5e51f40bd3b9 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
)
