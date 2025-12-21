module github.com/sagernet/sing-box

go 1.24.7

require (
	github.com/anthropics/anthropic-sdk-go v1.19.0
	github.com/anytls/sing-anytls v0.0.11
	github.com/caddyserver/certmagic v0.25.0
	github.com/coder/websocket v1.8.14
	github.com/cretz/bine v0.2.0
	github.com/database64128/tfo-go/v2 v2.3.1
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/render v1.0.3
	github.com/godbus/dbus/v5 v5.2.1
	github.com/gofrs/uuid/v5 v5.4.0
	github.com/insomniacslk/dhcp v0.0.0-20251020182700-175e84fbb167
	github.com/keybase/go-keychain v0.0.1
	github.com/libdns/alidns v1.0.6-beta.3
	github.com/libdns/cloudflare v0.2.2
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/metacubex/utls v1.8.4
	github.com/mholt/acmez/v3 v3.1.4
	github.com/miekg/dns v1.1.69
	github.com/openai/openai-go/v3 v3.15.0
	github.com/oschwald/maxminddb-golang v1.13.1
	github.com/sagernet/asc-go v0.0.0-20241217030726-d563060fe4e1
	github.com/sagernet/bbolt v0.0.0-20231014093535-ea5cb2fe9f0a
	github.com/sagernet/cors v1.2.1
	github.com/sagernet/cronet-go v0.0.0-20251220122645-b05b5c41614a
	github.com/sagernet/cronet-go/all v0.0.0-20251220122645-b05b5c41614a
	github.com/sagernet/fswatch v0.1.1
	github.com/sagernet/gomobile v0.1.10
	github.com/sagernet/gvisor v0.0.0-20250811.0-sing-box-mod.1
	github.com/sagernet/quic-go v0.58.0-sing-box-mod.1
	github.com/sagernet/sing v0.8.0-beta.7
	github.com/sagernet/sing-mux v0.3.4
	github.com/sagernet/sing-quic v0.6.0-beta.7
	github.com/sagernet/sing-shadowsocks v0.2.8
	github.com/sagernet/sing-shadowsocks2 v0.2.1
	github.com/sagernet/sing-shadowtls v0.2.1-0.20250503051639-fcd445d33c11
	github.com/sagernet/sing-tun v0.8.0-beta.11.0.20251201004738-e9e3fbf0c15e
	github.com/sagernet/sing-vmess v0.2.8-0.20250909125414-3aed155119a1
	github.com/sagernet/smux v1.5.50-sing-box-mod.1
	github.com/sagernet/tailscale v1.86.5-sing-box-1.13-mod.4
	github.com/sagernet/wireguard-go v0.0.2-beta.1.0.20250917110311-16510ac47288
	github.com/sagernet/ws v0.0.0-20231204124109-acfe8907c854
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	github.com/vishvananda/netns v0.0.5
	go.uber.org/zap v1.27.1
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba
	golang.org/x/crypto v0.46.0
	golang.org/x/exp v0.0.0-20251219203646-944ab1f22d93
	golang.org/x/mod v0.31.0
	golang.org/x/net v0.48.0
	golang.org/x/sys v0.39.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.11
	howett.net/plist v1.0.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/akutz/memconn v0.1.0 // indirect
	github.com/alexbrainman/sspi v0.0.0-20231016080023-1a75b4708caa // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/caddyserver/zerossl v0.1.3 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/coreos/go-iptables v0.7.1-0.20240112124308-65c67c9f46e6 // indirect
	github.com/database64128/netx-go v0.1.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dblohm7/wingoes v0.0.0-20240119213807-a09d6be7affa // indirect
	github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1 // indirect
	github.com/digitalocean/go-smbios v0.0.0-20180907143718-390a4f403a8e // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gaissmai/bart v0.18.0 // indirect
	github.com/go-json-experiment/json v0.0.0-20250223041408-d3c622f1b874 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/nftables v0.2.1-0.20240414091927-5e242ec57806 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/hdevalence/ed25519consensus v0.2.0 // indirect
	github.com/illarion/gonotify/v3 v3.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jsimonetti/rtnetlink v1.4.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/libdns/libdns v1.1.1 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.3-0.20250113171957-fbb4dce95f42 // indirect
	github.com/mdlayher/sdnotify v1.0.0 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mitchellh/go-ps v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus-community/pro-bing v0.4.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/safchain/ethtool v0.3.0 // indirect
	github.com/sagernet/cronet-go/lib/android_386 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/android_amd64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/android_arm v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/android_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/darwin_amd64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/darwin_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/ios_amd64_simulator v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/ios_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/ios_arm64_simulator v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_386 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_386_musl v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_amd64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_amd64_musl v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_arm v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_arm64_musl v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/linux_arm_musl v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/tvos_amd64_simulator v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/tvos_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/tvos_arm64_simulator v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/windows_amd64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/cronet-go/lib/windows_arm64 v0.0.0-20251220122226-25b6d00c5b7e // indirect
	github.com/sagernet/netlink v0.0.0-20240612041022-b9a21c07ac6a // indirect
	github.com/sagernet/nftables v0.3.0-beta.4 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tailscale/certstore v0.1.1-0.20231202035212-d3fa0460f47e // indirect
	github.com/tailscale/go-winio v0.0.0-20231025203758-c4f33415bf55 // indirect
	github.com/tailscale/goupnp v1.0.1-0.20210804011211-c64d0f06ea05 // indirect
	github.com/tailscale/hujson v0.0.0-20221223112325-20486734a56a // indirect
	github.com/tailscale/netlink v1.1.1-0.20240822203006-4d49adab4de7 // indirect
	github.com/tailscale/peercred v0.0.0-20250107143737-35a0c7bd7edc // indirect
	github.com/tailscale/web-client-prebuilt v0.0.0-20250124233751-d4cd19a26976 // indirect
	github.com/tailscale/wireguard-go v0.0.0-20250716170648-1d0488a3d7da // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go4.org/mem v0.0.0-20240501181205-ae6ca9944745 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/term v0.38.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	golang.zx2c4.com/wireguard/windows v0.5.3 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)
