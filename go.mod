module github.com/sagernet/sing-box

go 1.18

require (
	berty.tech/go-libtor v1.0.385
	github.com/Dreamacro/clash v1.11.8
	github.com/caddyserver/certmagic v0.17.1
	github.com/cloudflare/circl v1.2.1-0.20220831060716-4cf0150356fc
	github.com/cretz/bine v0.2.0
	github.com/database64128/tfo-go v1.1.2
	github.com/dustin/go-humanize v1.0.0
	github.com/fsnotify/fsnotify v1.5.4
	github.com/go-chi/chi/v5 v5.0.7
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/render v1.0.2
	github.com/gofrs/uuid v4.3.0+incompatible
	github.com/hashicorp/yamux v0.1.1
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mholt/acmez v1.0.4
	github.com/miekg/dns v1.1.50
	github.com/oschwald/maxminddb-golang v1.10.0
	github.com/pires/go-proxyproto v0.6.2
	github.com/refraction-networking/utls v1.1.2
	github.com/sagernet/quic-go v0.0.0-20220818150011-de611ab3e2bb
	github.com/sagernet/sing v0.0.0-20220921101604-86d7d510231f
	github.com/sagernet/sing-dns v0.0.0-20220915084601-812e0864b45b
	github.com/sagernet/sing-shadowsocks v0.0.0-20220819002358-7461bb09a8f6
	github.com/sagernet/sing-tun v0.0.0-20220922083325-80ee99472704
	github.com/sagernet/sing-vmess v0.0.0-20220921140858-b6a1bdee672f
	github.com/sagernet/smux v0.0.0-20220831015742-e0f1988e3195
	github.com/sagernet/websocket v0.0.0-20220913015213-615516348b4e
	github.com/spf13/cobra v1.5.0
	github.com/stretchr/testify v1.8.0
	go.etcd.io/bbolt v1.3.6
	go.uber.org/atomic v1.10.0
	go4.org/netipx v0.0.0-20220812043211-3cc044ffd68d
	golang.org/x/crypto v0.0.0-20220919173607-35f4265a4bc0
	golang.org/x/net v0.0.0-20220909164309-bea034e7d591
	golang.org/x/sys v0.0.0-20220913120320-3275c407cedc
	golang.zx2c4.com/wireguard v0.0.0-20220829161405-d1d08426b27b
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
	gvisor.dev/gvisor v0.0.0-20220901235040-6ca97ef2ce1c
)

//replace github.com/sagernet/sing => ../sing

require (
	github.com/ajg/form v1.5.1 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/klauspost/cpuid/v2 v2.1.0 // indirect
	github.com/libdns/libdns v0.2.1 // indirect
	github.com/marten-seemann/qpack v0.2.1 // indirect
	github.com/marten-seemann/qtls-go1-18 v0.1.2 // indirect
	github.com/marten-seemann/qtls-go1-19 v0.1.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sagernet/abx-go v0.0.0-20220819185957-dba1257d738e // indirect
	github.com/sagernet/go-tun2socks v1.16.12-0.20220818015926-16cb67876a61 // indirect
	github.com/sagernet/netlink v0.0.0-20220905062125-8043b4a9aa97 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.22.0 // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.1.11-0.20220513221640-090b14e8501f // indirect
	golang.zx2c4.com/wintun v0.0.0-20211104114900-415007cec224 // indirect
	google.golang.org/genproto v0.0.0-20210722135532-667f2b7c528f // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)
