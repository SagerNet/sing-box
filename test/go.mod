module test

go 1.18

require github.com/sagernet/sing-box v0.0.0

replace github.com/sagernet/sing-box => ../

require (
	github.com/docker/docker v20.10.18+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/sagernet/sing v0.1.8-0.20230221060643-3401d210384b
	github.com/sagernet/sing-shadowsocks v0.1.2-0.20230221080503-769c01d6bba9
	github.com/spyzhov/ajson v0.7.1
	github.com/stretchr/testify v1.8.1
	go.uber.org/goleak v1.2.0
	golang.org/x/net v0.7.0
)

replace github.com/xtls/reality => github.com/nekohasekai/reality v0.0.0-20230225043811-04070a6bdbea

require (
	berty.tech/go-libtor v1.0.385 // indirect
	github.com/Dreamacro/clash v1.13.0 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/caddyserver/certmagic v0.17.2 // indirect
	github.com/cloudflare/circl v1.2.1-0.20221019164342-6ab4dfed8f3c // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-chi/chi/v5 v5.0.8 // indirect
	github.com/go-chi/cors v1.2.1 // indirect
	github.com/go-chi/render v1.0.2 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20230220010740-598984875576 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/mholt/acmez v1.1.0 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/moby/term v0.0.0-20221105221325-4eb28fa6025c // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nekohasekai/reality v0.0.0-20230225080858-d70c703b04cd // indirect
	github.com/onsi/ginkgo/v2 v2.2.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/oschwald/maxminddb-golang v1.10.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/pires/go-proxyproto v0.6.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-18 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-19 v0.2.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.1.0 // indirect
	github.com/sagernet/cloudflare-tls v0.0.0-20221031050923-d70792f4c3a0 // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/sagernet/quic-go v0.0.0-20230202071646-a8c8afb18b32 // indirect
	github.com/sagernet/sing-dns v0.1.4 // indirect
	github.com/sagernet/sing-shadowtls v0.0.0-20230221123345-78e50cd7b587 // indirect
	github.com/sagernet/sing-tun v0.1.1 // indirect
	github.com/sagernet/sing-vmess v0.1.2 // indirect
	github.com/sagernet/smux v0.0.0-20220831015742-e0f1988e3195 // indirect
	github.com/sagernet/tfo-go v0.0.0-20230207095944-549363a7327d // indirect
	github.com/sagernet/utls v0.0.0-20230225061716-536a007c8b01 // indirect
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e // indirect
	github.com/sagernet/wireguard-go v0.0.0-20221116151939-c99467f53f2c // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/u-root/uio v0.0.0-20230215032506-9aa6f7e2d72c // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	go4.org/netipx v0.0.0-20230125063823-8449b0a6169f // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/exp v0.0.0-20230213192124-5e25df0256eb // indirect
	golang.org/x/mod v0.6.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.2.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.53.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.4.0 // indirect
	gvisor.dev/gvisor v0.0.0-20220901235040-6ca97ef2ce1c // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)

//replace github.com/sagernet/sing => ../../sing
