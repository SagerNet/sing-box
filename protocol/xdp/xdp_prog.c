// SPDX-License-Identifier: GPL-2.0
//
// XDP program for sing-box AF_XDP packet interception.
//
// Uses LPM trie maps for route_address (whitelist) and route_exclude_address
// (blacklist) to decide which destination IPs to redirect to AF_XDP sockets.
// Non-TCP/UDP protocols (ARP, ICMP, etc.) always pass to the kernel stack.
// Kernel socket lookup (bpf_sk_lookup) protects existing kernel connections.
//
// Safety: default action is XDP_PASS. Packets are only redirected when
// they match route_address AND are not in route_exclude_address AND
// there is no kernel socket AND there is a registered AF_XDP socket.
//
// Compile:
//   clang -O2 -g -Wall -target bpf -c xdp_prog.c -o xdp_prog.o
//   llvm-strip -g xdp_prog.o

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// ---------------------------------------------------------------------------
// LPM Trie Key Structures
// ---------------------------------------------------------------------------

struct lpm_ipv4_key {
	__u32 prefixlen;
	__u8  addr[4];
};

struct lpm_ipv6_key {
	__u32 prefixlen;
	__u8  addr[16];
};

// ---------------------------------------------------------------------------
// BPF Maps
// ---------------------------------------------------------------------------

// AF_XDP socket map: key = RX queue index, value = XSK file descriptor.
struct {
	__uint(type, BPF_MAP_TYPE_XSKMAP);
	__uint(max_entries, 64);
	__type(key, __u32);
	__type(value, __u32);
} xsks_map SEC(".maps");

// route_address IPv4: destination prefixes to capture (whitelist).
// When empty (no entries), Go side populates with 0.0.0.0/0 to capture all.
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__type(key, struct lpm_ipv4_key);
	__type(value, __u8);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} route_ipv4_addrs SEC(".maps");

// route_address IPv6: destination prefixes to capture (whitelist).
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__type(key, struct lpm_ipv6_key);
	__type(value, __u8);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} route_ipv6_addrs SEC(".maps");

// route_exclude_address IPv4: destination prefixes to exclude (blacklist).
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__type(key, struct lpm_ipv4_key);
	__type(value, __u8);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} route_exclude_ipv4_addrs SEC(".maps");

// route_exclude_address IPv6: destination prefixes to exclude (blacklist).
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__type(key, struct lpm_ipv6_key);
	__type(value, __u8);
	__uint(map_flags, BPF_F_NO_PREALLOC);
} route_exclude_ipv6_addrs SEC(".maps");

// local_ipv4_hints / local_ipv6_hints: hash sets of the host's own IP
// addresses. Used to gate bpf_sk_lookup: only packets whose destination is a
// local IP can possibly have a kernel socket, so we skip the lookup for
// transit traffic (dst = remote host). Populated and updated from Go side.
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 256);
	__type(key, __u32);
	__type(value, __u8);
} local_ipv4_hints SEC(".maps");

struct ipv6_hint_key {
	__u8 addr[16];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 256);
	__type(key, struct ipv6_hint_key);
	__type(value, __u8);
} local_ipv6_hints SEC(".maps");

// ---------------------------------------------------------------------------
// Route matching helpers
// ---------------------------------------------------------------------------

// Check if the IPv4 destination address is in the route_exclude (blacklist).
static __always_inline int
is_ipv4_route_excluded(__be32 daddr)
{
	struct lpm_ipv4_key key = { .prefixlen = 32 };
	__builtin_memcpy(&key.addr, &daddr, 4);
	return bpf_map_lookup_elem(&route_exclude_ipv4_addrs, &key) != NULL;
}

// Check if the IPv4 destination address is in the route_address (whitelist).
static __always_inline int
is_ipv4_route_matched(__be32 daddr)
{
	struct lpm_ipv4_key key = { .prefixlen = 32 };
	__builtin_memcpy(&key.addr, &daddr, 4);
	return bpf_map_lookup_elem(&route_ipv4_addrs, &key) != NULL;
}

// Check if the IPv6 destination address is in the route_exclude (blacklist).
static __always_inline int
is_ipv6_route_excluded(const struct in6_addr *daddr)
{
	struct lpm_ipv6_key key = { .prefixlen = 128 };
	__builtin_memcpy(&key.addr, daddr, 16);
	return bpf_map_lookup_elem(&route_exclude_ipv6_addrs, &key) != NULL;
}

// Check if the IPv6 destination address is in the route_address (whitelist).
static __always_inline int
is_ipv6_route_matched(const struct in6_addr *daddr)
{
	struct lpm_ipv6_key key = { .prefixlen = 128 };
	__builtin_memcpy(&key.addr, daddr, 16);
	return bpf_map_lookup_elem(&route_ipv6_addrs, &key) != NULL;
}

// ---------------------------------------------------------------------------
// Kernel socket lookup helpers
//
// If the kernel already owns a socket for this packet (e.g. an outbound
// proxy connection initiated by sing-box through the kernel stack), the
// packet MUST be delivered to the kernel (XDP_PASS), otherwise the
// outbound connection will never receive its reply traffic.
//
// This is critical for single-NIC setups where both inbound (to-be-proxied)
// and outbound (proxy upstream) traffic share the same interface.
// ---------------------------------------------------------------------------

// Check if the kernel has a TCP socket matching this IPv4 4-tuple.
static __always_inline int
kernel_has_ipv4_tcp_socket(struct xdp_md *ctx,
			   __be32 saddr, __be32 daddr,
			   __be16 sport, __be16 dport)
{
	struct bpf_sock_tuple tuple;
	struct bpf_sock *sk;

	__builtin_memset(&tuple, 0, sizeof(tuple));
	tuple.ipv4.saddr = saddr;
	tuple.ipv4.daddr = daddr;
	tuple.ipv4.sport = sport;
	tuple.ipv4.dport = dport;

	sk = bpf_sk_lookup_tcp(ctx, &tuple, sizeof(tuple.ipv4),
			       BPF_F_CURRENT_NETNS, 0);
	if (sk) {
		bpf_sk_release(sk);
		return 1;
	}
	return 0;
}

// Check if the kernel has a UDP socket matching this IPv4 4-tuple.
static __always_inline int
kernel_has_ipv4_udp_socket(struct xdp_md *ctx,
			   __be32 saddr, __be32 daddr,
			   __be16 sport, __be16 dport)
{
	struct bpf_sock_tuple tuple;
	struct bpf_sock *sk;

	__builtin_memset(&tuple, 0, sizeof(tuple));
	tuple.ipv4.saddr = saddr;
	tuple.ipv4.daddr = daddr;
	tuple.ipv4.sport = sport;
	tuple.ipv4.dport = dport;

	sk = bpf_sk_lookup_udp(ctx, &tuple, sizeof(tuple.ipv4),
			       BPF_F_CURRENT_NETNS, 0);
	if (sk) {
		bpf_sk_release(sk);
		return 1;
	}
	return 0;
}

// Check if the kernel has a TCP socket matching this IPv6 4-tuple.
static __always_inline int
kernel_has_ipv6_tcp_socket(struct xdp_md *ctx,
			   const struct in6_addr *saddr,
			   const struct in6_addr *daddr,
			   __be16 sport, __be16 dport)
{
	struct bpf_sock_tuple tuple;
	struct bpf_sock *sk;

	__builtin_memset(&tuple, 0, sizeof(tuple));
	__builtin_memcpy(&tuple.ipv6.saddr, saddr, 16);
	__builtin_memcpy(&tuple.ipv6.daddr, daddr, 16);
	tuple.ipv6.sport = sport;
	tuple.ipv6.dport = dport;

	sk = bpf_sk_lookup_tcp(ctx, &tuple, sizeof(tuple.ipv6),
			       BPF_F_CURRENT_NETNS, 0);
	if (sk) {
		bpf_sk_release(sk);
		return 1;
	}
	return 0;
}

// Check if the kernel has a UDP socket matching this IPv6 4-tuple.
static __always_inline int
kernel_has_ipv6_udp_socket(struct xdp_md *ctx,
			   const struct in6_addr *saddr,
			   const struct in6_addr *daddr,
			   __be16 sport, __be16 dport)
{
	struct bpf_sock_tuple tuple;
	struct bpf_sock *sk;

	__builtin_memset(&tuple, 0, sizeof(tuple));
	__builtin_memcpy(&tuple.ipv6.saddr, saddr, 16);
	__builtin_memcpy(&tuple.ipv6.daddr, daddr, 16);
	tuple.ipv6.sport = sport;
	tuple.ipv6.dport = dport;

	sk = bpf_sk_lookup_udp(ctx, &tuple, sizeof(tuple.ipv6),
			       BPF_F_CURRENT_NETNS, 0);
	if (sk) {
		bpf_sk_release(sk);
		return 1;
	}
	return 0;
}

// ---------------------------------------------------------------------------
// Process IPv4 packet. Returns XDP action.
// ---------------------------------------------------------------------------
static __always_inline int
process_ipv4(struct xdp_md *ctx, void *l3_hdr, void *data_end, __u32 rx_queue_index)
{
	struct iphdr *iph = l3_hdr;

	// Validate IPv4 header is accessible
	if ((void *)(iph + 1) > data_end)
		return XDP_PASS;

	// Skip limited broadcast (255.255.255.255) and multicast (224.0.0.0/4)
	if (iph->daddr == bpf_htonl(0xFFFFFFFF))
		return XDP_PASS;
	if ((iph->daddr & bpf_htonl(0xF0000000)) == bpf_htonl(0xE0000000))
		return XDP_PASS;

	// Check route_exclude_address (blacklist) — destination only
	if (is_ipv4_route_excluded(iph->daddr))
		return XDP_PASS;

	// Check route_address (whitelist) — destination only
	if (!is_ipv4_route_matched(iph->daddr))
		return XDP_PASS;

	// Only intercept TCP and UDP
	__u8 proto = iph->protocol;
	if (proto != IPPROTO_TCP && proto != IPPROTO_UDP)
		return XDP_PASS;

	// Compute L4 header offset using IHL field
	__u32 ihl = iph->ihl;
	if (ihl < 5)
		return XDP_PASS;

	void *l4_hdr = (void *)iph + (ihl << 2);

	// bpf_sk_lookup optimization: transit traffic (dst = remote host)
	// cannot have kernel sockets on this machine. Only check when the
	// destination IP is one of the host's own addresses (hint map hit).
	int local4 = bpf_map_lookup_elem(&local_ipv4_hints, &iph->daddr) != NULL;

	if (proto == IPPROTO_TCP) {
		struct tcphdr *tcph = l4_hdr;
		if ((void *)(tcph + 1) > data_end)
			return XDP_PASS;
		if (local4 && kernel_has_ipv4_tcp_socket(ctx, iph->saddr, iph->daddr,
						       tcph->source, tcph->dest))
			return XDP_PASS;
	} else { // UDP
		struct udphdr *udph = l4_hdr;
		if ((void *)(udph + 1) > data_end)
			return XDP_PASS;
		if (local4 && kernel_has_ipv4_udp_socket(ctx, iph->saddr, iph->daddr,
						       udph->source, udph->dest))
			return XDP_PASS;
	}

	// Redirect to AF_XDP socket; falls back to XDP_PASS if no socket
	return bpf_redirect_map(&xsks_map, rx_queue_index, XDP_PASS);
}

// ---------------------------------------------------------------------------
// Process IPv6 packet. Returns XDP action.
// ---------------------------------------------------------------------------
static __always_inline int
process_ipv6(struct xdp_md *ctx, void *l3_hdr, void *data_end, __u32 rx_queue_index)
{
	struct ipv6hdr *ip6h = l3_hdr;

	if ((void *)(ip6h + 1) > data_end)
		return XDP_PASS;

	// Skip multicast destinations (ff00::/8)
	if (ip6h->daddr.s6_addr[0] == 0xFF)
		return XDP_PASS;

	// Check route_exclude_address (blacklist) — destination only
	if (is_ipv6_route_excluded(&ip6h->daddr))
		return XDP_PASS;

	// Check route_address (whitelist) — destination only
	if (!is_ipv6_route_matched(&ip6h->daddr))
		return XDP_PASS;

	// Only intercept TCP and UDP; extension headers are not parsed
	// (packets with extension headers will XDP_PASS, which is safe).
	__u8 nexthdr = ip6h->nexthdr;
	if (nexthdr != IPPROTO_TCP && nexthdr != IPPROTO_UDP)
		return XDP_PASS;

	void *l4_hdr = (void *)(ip6h + 1);

	// Compute hint once for both TCP and UDP branches.
	struct ipv6_hint_key hkey6;
	__builtin_memcpy(&hkey6.addr, &ip6h->daddr, 16);
	int local6 = bpf_map_lookup_elem(&local_ipv6_hints, &hkey6) != NULL;

	if (nexthdr == IPPROTO_TCP) {
		struct tcphdr *tcph = l4_hdr;
		if ((void *)(tcph + 1) > data_end)
			return XDP_PASS;
		if (local6 && kernel_has_ipv6_tcp_socket(ctx, &ip6h->saddr, &ip6h->daddr,
						       tcph->source, tcph->dest))
			return XDP_PASS;
	} else { // UDP
		struct udphdr *udph = l4_hdr;
		if ((void *)(udph + 1) > data_end)
			return XDP_PASS;
		if (local6 && kernel_has_ipv6_udp_socket(ctx, &ip6h->saddr, &ip6h->daddr,
						       udph->source, udph->dest))
			return XDP_PASS;
	}

	return bpf_redirect_map(&xsks_map, rx_queue_index, XDP_PASS);
}

// ---------------------------------------------------------------------------
// XDP entry point
// ---------------------------------------------------------------------------
SEC("xdp")
int xsk_def_prog(struct xdp_md *ctx)
{
	void *data     = (void *)(long)ctx->data;
	void *data_end = (void *)(long)ctx->data_end;

	// Parse Ethernet header
	struct ethhdr *eth = data;
	if ((void *)(eth + 1) > data_end)
		return XDP_PASS;

	__be16 eth_proto = eth->h_proto;
	void *l3_hdr = (void *)(eth + 1);

	// Handle single 802.1Q VLAN tag
	if (eth_proto == bpf_htons(ETH_P_8021Q) ||
	    eth_proto == bpf_htons(0x88A8)) {  // Q-in-Q outer tag
		if (l3_hdr + 4 > data_end)
			return XDP_PASS;
		eth_proto = *(__be16 *)(l3_hdr + 2);
		l3_hdr += 4;
	}

	__u32 rx_queue = ctx->rx_queue_index;

	if (eth_proto == bpf_htons(ETH_P_IP))
		return process_ipv4(ctx, l3_hdr, data_end, rx_queue);

	if (eth_proto == bpf_htons(ETH_P_IPV6))
		return process_ipv6(ctx, l3_hdr, data_end, rx_queue);

	// Non-IP (ARP, LLDP, etc.) → pass to kernel
	return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
