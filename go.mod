module github.com/sagernet/sing-box

go 1.18

require (
	berty.tech/go-libtor v1.0.385
	github.com/Dreamacro/clash v1.15.0
	github.com/caddyserver/certmagic v0.17.2
	github.com/cretz/bine v0.2.0
	github.com/dustin/go-humanize v1.0.1
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/render v1.0.2
	github.com/gofrs/uuid/v5 v5.0.0
	github.com/hashicorp/yamux v0.1.1
	github.com/insomniacslk/dhcp v0.0.0-20230407062729-974c6f05fe16
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mholt/acmez v1.1.0
	github.com/miekg/dns v1.1.53
	github.com/ooni/go-libtor v1.1.7
	github.com/oschwald/maxminddb-golang v1.10.0
	github.com/pires/go-proxyproto v0.7.0
	github.com/sagernet/cloudflare-tls v0.0.0-20221031050923-d70792f4c3a0
	github.com/sagernet/gomobile v0.0.0-20230413023804-244d7ff07035
	github.com/sagernet/quic-go v0.0.0-20230202071646-a8c8afb18b32
	github.com/sagernet/reality v0.0.0-20230406110435-ee17307e7691
	github.com/sagernet/sing v0.2.4-0.20230418095640-3b5e6c1812d3
	github.com/sagernet/sing-dns v0.1.5-0.20230418025317-8a132998b322
	github.com/sagernet/sing-shadowsocks v0.2.2-0.20230418025154-6114beeeba6d
	github.com/sagernet/sing-shadowtls v0.1.2-0.20230417103049-4f682e05f19b
	github.com/sagernet/sing-tun v0.1.4-0.20230419061614-d744d03d9302
	github.com/sagernet/sing-vmess v0.1.5-0.20230417103030-8c3070ae3fb3
	github.com/sagernet/smux v0.0.0-20230312102458-337ec2a5af37
	github.com/sagernet/tfo-go v0.0.0-20230303015439-ffcfd8c41cf9
	github.com/sagernet/utls v0.0.0-20230309024959-6732c2ab36f2
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e
	github.com/sagernet/wireguard-go v0.0.0-20221116151939-c99467f53f2c
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.8.2
	go.etcd.io/bbolt v1.3.7
	go.uber.org/zap v1.24.0
	go4.org/netipx v0.0.0-20230303233057-f1b76eb4bb35
	golang.org/x/crypto v0.8.0
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
	golang.org/x/net v0.9.0
	golang.org/x/sys v0.7.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230215201556-9c5414ab4bde
	google.golang.org/grpc v1.54.0
	google.golang.org/protobuf v1.30.0
	gvisor.dev/gvisor v0.0.0-20220901235040-6ca97ef2ce1c
)

//replace github.com/sagernet/sing => ../sing

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/cloudflare/circl v1.2.1-0.20221019164342-6ab4dfed8f3c // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo/v2 v2.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-18 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-19 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.1.0 // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)
