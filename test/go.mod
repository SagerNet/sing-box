module test

go 1.18

require github.com/sagernet/sing-box v0.0.0

replace github.com/sagernet/sing-box => ../

require (
	github.com/docker/docker v20.10.18+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/gofrs/uuid v4.3.1+incompatible
	github.com/sagernet/sing v0.0.0-20221008120626-60a9910eefe4
	github.com/sagernet/sing-shadowsocks v0.0.0-20220819002358-7461bb09a8f6
	github.com/spyzhov/ajson v0.7.1
	github.com/stretchr/testify v1.8.1
	go.uber.org/goleak v1.2.0
	golang.org/x/net v0.2.0
)

require (
	berty.tech/go-libtor v1.0.385 // indirect
	github.com/Dreamacro/clash v1.11.12 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/caddyserver/certmagic v0.17.2 // indirect
	github.com/cloudflare/circl v1.2.1-0.20221019164342-6ab4dfed8f3c // indirect
	github.com/cretz/bine v0.2.0 // indirect
	github.com/database64128/tfo-go/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-chi/chi/v5 v5.0.7 // indirect
	github.com/go-chi/cors v1.2.1 // indirect
	github.com/go-chi/render v1.0.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/klauspost/compress v1.15.12 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/marten-seemann/qpack v0.3.0 // indirect
	github.com/marten-seemann/qtls-go1-18 v0.1.3 // indirect
	github.com/marten-seemann/qtls-go1-19 v0.1.1 // indirect
	github.com/mholt/acmez v1.0.4 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/moby/term v0.0.0-20221105221325-4eb28fa6025c // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/oschwald/maxminddb-golang v1.10.0 // indirect
	github.com/pires/go-proxyproto v0.6.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/refraction-networking/utls v1.2.0 // indirect
	github.com/sagernet/abx-go v0.0.0-20220819185957-dba1257d738e // indirect
	github.com/sagernet/cloudflare-tls v0.0.0-20221031050923-d70792f4c3a0 // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/sagernet/quic-go v0.0.0-20221108053023-645bcc4f9b15 // indirect
	github.com/sagernet/sing-dns v0.0.0-20221113031420-c6aaf2ea4b10 // indirect
	github.com/sagernet/sing-tun v0.0.0-20221104121441-66c48a57776f // indirect
	github.com/sagernet/sing-vmess v0.0.0-20221109021549-b446d5bdddf0 // indirect
	github.com/sagernet/smux v0.0.0-20220831015742-e0f1988e3195 // indirect
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e // indirect
	github.com/sagernet/wireguard-go v0.0.0-20221116151939-c99467f53f2c // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.23.0 // indirect
	go4.org/netipx v0.0.0-20220925034521-797b0c90d8ab // indirect
	golang.org/x/crypto v0.3.0 // indirect
	golang.org/x/exp v0.0.0-20221028150844-83b7d23a625f // indirect
	golang.org/x/mod v0.6.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.2.0 // indirect
	google.golang.org/genproto v0.0.0-20210722135532-667f2b7c528f // indirect
	google.golang.org/grpc v1.51.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.4.0 // indirect
	gvisor.dev/gvisor v0.0.0-20220901235040-6ca97ef2ce1c // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)

//replace github.com/sagernet/sing => ../../sing
