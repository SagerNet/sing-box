module test

go 1.20

require github.com/sagernet/sing-box v0.0.0

replace github.com/sagernet/sing-box => ../

require (
	github.com/docker/docker v24.0.5+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/gofrs/uuid/v5 v5.0.0
	github.com/sagernet/sing v0.2.10-0.20230830132630-30bf19f2833c
	github.com/sagernet/sing-shadowsocks v0.2.4
	github.com/sagernet/sing-shadowsocks2 v0.1.3
	github.com/spyzhov/ajson v0.9.0
	github.com/stretchr/testify v1.8.4
	go.uber.org/goleak v1.2.1
	golang.org/x/net v0.14.0
)

require (
	berty.tech/go-libtor v1.0.385 // indirect
	github.com/Dreamacro/clash v1.17.0 // indirect
	github.com/Dreamacro/protobytes v0.0.0-20230617041236-6500a9f4f158 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/caddyserver/certmagic v0.19.2 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-chi/chi/v5 v5.0.10 // indirect
	github.com/go-chi/cors v1.2.1 // indirect
	github.com/go-chi/render v1.0.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20230816195147-b3ca2534940d // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/mholt/acmez v1.2.0 // indirect
	github.com/miekg/dns v1.1.55 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/onsi/ginkgo/v2 v2.9.5 // indirect
	github.com/ooni/go-libtor v1.1.8 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/oschwald/maxminddb-golang v1.12.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.14 // indirect
	github.com/pires/go-proxyproto v0.7.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/qtls-go1-20 v0.3.3 // indirect
	github.com/sagernet/cloudflare-tls v0.0.0-20230829051644-4a68352d0c4a // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/gvisor v0.0.0-20230627031050-1ab0276e0dd2 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/sagernet/quic-go v0.0.0-20230831052420-45809eee2e86 // indirect
	github.com/sagernet/reality v0.0.0-20230406110435-ee17307e7691 // indirect
	github.com/sagernet/sing-dns v0.1.9-0.20230824120133-4d5cbceb40c1 // indirect
	github.com/sagernet/sing-mux v0.1.3-0.20230830095209-2a10ebd53ba8 // indirect
	github.com/sagernet/sing-shadowtls v0.1.4 // indirect
	github.com/sagernet/sing-tun v0.1.12-0.20230821065522-7545dc2d5641 // indirect
	github.com/sagernet/sing-vmess v0.1.7 // indirect
	github.com/sagernet/smux v0.0.0-20230312102458-337ec2a5af37 // indirect
	github.com/sagernet/tfo-go v0.0.0-20230816093905-5a5c285d44a6 // indirect
	github.com/sagernet/utls v0.0.0-20230309024959-6732c2ab36f2 // indirect
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e // indirect
	github.com/sagernet/wireguard-go v0.0.0-20230807125731-5d4a7ef2dc5f // indirect
	github.com/scjalliance/comshim v0.0.0-20230315213746-5e51f40bd3b9 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	go4.org/netipx v0.0.0-20230824141953-6213f710f925 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.12.1-0.20230815132531-74c255bcf846 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19 // indirect
	google.golang.org/grpc v1.57.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.4.0 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
)
