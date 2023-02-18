module github.com/sagernet/sing-box

go 1.18

require (
	berty.tech/go-libtor v1.0.385
	github.com/Dreamacro/clash v1.13.0
	github.com/caddyserver/certmagic v0.17.2
	github.com/cretz/bine v0.2.0
	github.com/dustin/go-humanize v1.0.1
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/render v1.0.2
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/hashicorp/yamux v0.1.1
	github.com/insomniacslk/dhcp v0.0.0-20221215072855-de60144f33f8
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mholt/acmez v1.1.0
	github.com/miekg/dns v1.1.50
	github.com/oschwald/maxminddb-golang v1.10.0
	github.com/pires/go-proxyproto v0.6.2
	github.com/refraction-networking/utls v1.2.2
	github.com/sagernet/cloudflare-tls v0.0.0-20221031050923-d70792f4c3a0
	github.com/sagernet/quic-go v0.0.0-20230202071646-a8c8afb18b32
	github.com/sagernet/sing v0.1.7-0.20230209132010-5f1ef3441c13
	github.com/sagernet/sing-dns v0.1.2-0.20230209132355-3c2e2957b455
	github.com/sagernet/sing-shadowsocks v0.1.1-0.20230202035033-e3123545f2f7
	github.com/sagernet/sing-tun v0.1.1
	github.com/sagernet/sing-vmess v0.1.1-0.20230212211128-cb4e47dd0acb
	github.com/sagernet/smux v0.0.0-20220831015742-e0f1988e3195
	github.com/sagernet/tfo-go v0.0.0-20230207095944-549363a7327d
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e
	github.com/sagernet/wireguard-go v0.0.0-20221116151939-c99467f53f2c
	github.com/spf13/cobra v1.6.1
	github.com/stretchr/testify v1.8.1
	go.etcd.io/bbolt v1.3.7
	go.uber.org/atomic v1.10.0
	go.uber.org/zap v1.24.0
	go4.org/netipx v0.0.0-20230125063823-8449b0a6169f
	golang.org/x/crypto v0.6.0
	golang.org/x/exp v0.0.0-20230213192124-5e25df0256eb
	golang.org/x/net v0.7.0
	golang.org/x/sys v0.5.0
	google.golang.org/grpc v1.53.0
	google.golang.org/protobuf v1.28.1
	gvisor.dev/gvisor v0.0.0-20220901235040-6ca97ef2ce1c
)

//replace github.com/sagernet/sing => ../sing

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/cloudflare/circl v1.2.1-0.20221019164342-6ab4dfed8f3c // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo/v2 v2.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-18 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-19 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.1.0 // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/u-root/uio v0.0.0-20221213070652-c3537552635f // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/mod v0.6.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.2.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)
