//go:build cgo

package systemconfig

/*
#include <dlfcn.h>
#include <notify.h>
#include <stdint.h>
#include <string.h>
#include <netinet/in.h>
#include <sys/socket.h>

// dnsinfo.h is not shipped in any SDK. The layouts below are DNSINFO_VERSION
// 20170629 from apple-oss-distributions/configd (#pragma pack(4)), the format
// libsystem_configuration unpacks into at runtime. dns_configuration_copy,
// dns_configuration_free and dns_configuration_notify_key are private
// libSystem exports. cgo silently drops packed struct fields that fall on
// unaligned offsets.

#pragma pack(4)
typedef struct {
	struct in_addr address;
	struct in_addr mask;
} box_dns_sortaddr_t;

typedef struct {
	char *domain;
	int32_t n_nameserver;
	struct sockaddr **nameserver;
	uint16_t port;
	int32_t n_search;
	char **search;
	int32_t n_sortaddr;
	box_dns_sortaddr_t **sortaddr;
	char *options;
	uint32_t timeout;
	uint32_t search_order;
	uint32_t if_index;
	uint32_t flags;
	uint32_t reach_flags;
	uint32_t service_identifier;
	char *cid;
	char *if_name;
} box_dns_resolver_t;

typedef struct {
	int32_t n_resolver;
	box_dns_resolver_t **resolver;
	int32_t n_scoped_resolver;
	box_dns_resolver_t **scoped_resolver;
	uint64_t generation;
	int32_t n_service_specific_resolver;
	box_dns_resolver_t **service_specific_resolver;
	uint32_t version;
} box_dns_config_t;
#pragma pack()

static box_dns_config_t *(*box_dns_configuration_copy)(void);
static void (*box_dns_configuration_free)(box_dns_config_t *);

static void box_reverse_string(char *s) {
	size_t length = strlen(s);
	for (size_t i = 0; i < length / 2; i++) {
		char tmp = s[i];
		s[i] = s[length - 1 - i];
		s[length - 1 - i] = tmp;
	}
}

static int box_dnsinfo_load(void) {
	if (box_dns_configuration_copy != NULL && box_dns_configuration_free != NULL) {
		return 1;
	}
	char copy_name[] = "ypoc_noitarugifnoc_snd";
	char free_name[] = "eerf_noitarugifnoc_snd";
	box_reverse_string(copy_name);
	box_reverse_string(free_name);
	box_dns_configuration_copy = (box_dns_config_t * (*)(void)) dlsym(RTLD_DEFAULT, copy_name);
	box_dns_configuration_free = (void (*)(box_dns_config_t *))dlsym(RTLD_DEFAULT, free_name);
	return box_dns_configuration_copy != NULL && box_dns_configuration_free != NULL;
}

static box_dns_config_t *box_dnsinfo_copy(void) {
	return box_dns_configuration_copy();
}

static void box_dnsinfo_free(box_dns_config_t *config) {
	box_dns_configuration_free(config);
}

static const char *box_dnsinfo_notify_key(void) {
	const char *(*notify_key)(void) = (const char *(*)(void))dlsym(RTLD_DEFAULT, "dns_configuration_notify_key");
	if (notify_key != NULL) {
		return notify_key();
	}
	return "com.apple.system.SystemConfiguration.dns_configuration";
}

static box_dns_resolver_t *box_dnsinfo_default_resolver(box_dns_config_t *config, int32_t index) {
	return config->resolver[index];
}

static box_dns_resolver_t *box_dnsinfo_scoped_resolver(box_dns_config_t *config, int32_t index) {
	return config->scoped_resolver[index];
}

static struct sockaddr *box_dnsinfo_nameserver(box_dns_resolver_t *resolver, int32_t index) {
	return resolver->nameserver[index];
}

static const char *box_dnsinfo_search_domain(box_dns_resolver_t *resolver, int32_t index) {
	return resolver->search[index];
}
*/
import "C"

import (
	"context"
	"encoding/binary"
	"net/netip"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

type Source struct {
	interfaceMonitor tun.DefaultInterfaceMonitor
	access           sync.Mutex
	notifyToken      C.int
	notifyValid      bool
	stale            bool
	interfaceIndex   int
	config           *Config
}

func NewSource(ctx context.Context) *Source {
	source := &Source{
		interfaceMonitor: service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor(),
	}
	if C.box_dnsinfo_load() != 0 {
		var token C.int
		if C.notify_register_check(C.box_dnsinfo_notify_key(), &token) == 0 {
			source.notifyToken = token
			source.notifyValid = true
		}
	}
	return source
}

func (s *Source) Configuration() *Config {
	interfaceIndex := s.defaultInterfaceIndex()
	s.access.Lock()
	defer s.access.Unlock()
	interfaceChanged := s.interfaceIndex != interfaceIndex
	s.interfaceIndex = interfaceIndex
	changed := s.changedLocked()
	if s.config != nil && !s.stale && !interfaceChanged && !changed {
		return s.config
	}
	s.stale = false
	systemInfo := copyDNSInfo()
	if systemInfo == nil {
		if s.config == nil {
			s.config = new(dnsInfoConfig).build(interfaceIndex)
		}
		return s.config
	}
	config := systemInfo.build(interfaceIndex)
	if s.config != nil && config.Equal(s.config) {
		return s.config
	}
	s.config = config
	return config
}

func (s *Source) changedLocked() bool {
	if !s.notifyValid {
		return true
	}
	var changed C.int
	status := C.notify_check(s.notifyToken, &changed)
	if status != 0 {
		return true
	}
	return changed != 0
}

func (s *Source) Reset() {
	s.access.Lock()
	s.stale = true
	s.access.Unlock()
}

func (s *Source) Close() error {
	s.access.Lock()
	defer s.access.Unlock()
	if s.notifyValid {
		C.notify_cancel(s.notifyToken)
		s.notifyValid = false
	}
	return nil
}

func (s *Source) defaultInterfaceIndex() int {
	if s.interfaceMonitor == nil {
		return 0
	}
	defaultInterface := s.interfaceMonitor.DefaultInterface()
	if defaultInterface == nil {
		return 0
	}
	return defaultInterface.Index
}

type dnsInfoResolver struct {
	interfaceIndex int
	domain         string
	servers        []M.Socksaddr
	search         []string
	timeout        time.Duration
}

type dnsInfoConfig struct {
	resolvers       []dnsInfoResolver
	scopedResolvers []dnsInfoResolver
}

func (c *dnsInfoConfig) build(interfaceIndex int) *Config {
	var selected dnsInfoResolver
	if interfaceIndex != 0 {
		selected = common.Find(c.scopedResolvers, func(it dnsInfoResolver) bool {
			return it.interfaceIndex == interfaceIndex && len(it.servers) > 0
		})
	}
	if len(selected.servers) == 0 {
		selected = common.Find(c.resolvers, func(it dnsInfoResolver) bool {
			return it.domain == "" && len(it.servers) > 0
		})
	}
	config := &Config{
		Ndots:    1,
		Timeout:  5 * time.Second,
		Attempts: 2,
	}
	if len(selected.servers) == 0 {
		config.Servers = defaultServers
		config.Search = defaultSearch()
		return config
	}
	config.Servers = selected.servers
	if len(selected.search) > 0 {
		config.Search = selected.search
	} else {
		config.Search = defaultSearch()
	}
	if selected.timeout > 0 {
		config.Timeout = selected.timeout
	}
	return config
}

func copyDNSInfo() *dnsInfoConfig {
	if C.box_dnsinfo_load() == 0 {
		return nil
	}
	rawConfig := C.box_dnsinfo_copy()
	if rawConfig == nil {
		return nil
	}
	defer C.box_dnsinfo_free(rawConfig)
	systemInfo := new(dnsInfoConfig)
	for i := C.int32_t(0); i < rawConfig.n_resolver; i++ {
		rawResolver := C.box_dnsinfo_default_resolver(rawConfig, i)
		if rawResolver == nil {
			continue
		}
		systemInfo.resolvers = append(systemInfo.resolvers, parseResolver(rawResolver))
	}
	for i := C.int32_t(0); i < rawConfig.n_scoped_resolver; i++ {
		rawResolver := C.box_dnsinfo_scoped_resolver(rawConfig, i)
		if rawResolver == nil {
			continue
		}
		systemInfo.scopedResolvers = append(systemInfo.scopedResolvers, parseResolver(rawResolver))
	}
	return systemInfo
}

func parseResolver(rawResolver *C.box_dns_resolver_t) dnsInfoResolver {
	resolver := dnsInfoResolver{
		interfaceIndex: int(rawResolver.if_index),
		domain:         C.GoString(rawResolver.domain),
		timeout:        time.Duration(rawResolver.timeout) * time.Second,
	}
	interfaceName := C.GoString(rawResolver.if_name)
	resolverPort := uint16(rawResolver.port)
	if resolverPort == 0 {
		resolverPort = 53
	}
	for i := C.int32_t(0); i < rawResolver.n_nameserver; i++ {
		rawSockaddr := C.box_dnsinfo_nameserver(rawResolver, i)
		if rawSockaddr == nil {
			continue
		}
		serverAddr, loaded := parseSockaddr(rawSockaddr, resolverPort, interfaceName)
		if !loaded {
			continue
		}
		resolver.servers = append(resolver.servers, M.SocksaddrFromNetIP(serverAddr))
	}
	for i := C.int32_t(0); i < rawResolver.n_search; i++ {
		searchDomain := C.GoString(C.box_dnsinfo_search_domain(rawResolver, i))
		if searchDomain == "" {
			continue
		}
		searchDomain = mDNS.Fqdn(searchDomain)
		if searchDomain == "." {
			continue
		}
		resolver.search = append(resolver.search, searchDomain)
	}
	return resolver
}

func parseSockaddr(rawSockaddr *C.struct_sockaddr, fallbackPort uint16, zone string) (netip.AddrPort, bool) {
	switch rawSockaddr.sa_family {
	case C.AF_INET:
		sockaddrInet := (*C.struct_sockaddr_in)(unsafe.Pointer(rawSockaddr))
		addr := netip.AddrFrom4(*(*[4]byte)(unsafe.Pointer(&sockaddrInet.sin_addr)))
		return netip.AddrPortFrom(addr, sockaddrPort(unsafe.Pointer(&sockaddrInet.sin_port), fallbackPort)), true
	case C.AF_INET6:
		sockaddrInet6 := (*C.struct_sockaddr_in6)(unsafe.Pointer(rawSockaddr))
		addr := netip.AddrFrom16(*(*[16]byte)(unsafe.Pointer(&sockaddrInet6.sin6_addr)))
		if addr.IsLinkLocalUnicast() {
			scopeId := uint32(sockaddrInet6.sin6_scope_id)
			if zone == "" && scopeId != 0 {
				zone = strconv.FormatUint(uint64(scopeId), 10)
			}
			if zone != "" {
				addr = addr.WithZone(zone)
			}
		}
		return netip.AddrPortFrom(addr, sockaddrPort(unsafe.Pointer(&sockaddrInet6.sin6_port), fallbackPort)), true
	default:
		return netip.AddrPort{}, false
	}
}

func sockaddrPort(rawPort unsafe.Pointer, fallbackPort uint16) uint16 {
	port := binary.BigEndian.Uint16((*[2]byte)(rawPort)[:])
	if port == 0 {
		return fallbackPort
	}
	return port
}
