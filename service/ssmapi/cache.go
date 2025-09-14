package ssmapi

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/service/filemanager"
)

type Cache struct {
	Endpoints *badjson.TypedMap[string, *EndpointCache] `json:"endpoints"`
}

type EndpointCache struct {
	GlobalUplink          int64                             `json:"global_uplink"`
	GlobalDownlink        int64                             `json:"global_downlink"`
	GlobalUplinkPackets   int64                             `json:"global_uplink_packets"`
	GlobalDownlinkPackets int64                             `json:"global_downlink_packets"`
	GlobalTCPSessions     int64                             `json:"global_tcp_sessions"`
	GlobalUDPSessions     int64                             `json:"global_udp_sessions"`
	UserUplink            *badjson.TypedMap[string, int64]  `json:"user_uplink"`
	UserDownlink          *badjson.TypedMap[string, int64]  `json:"user_downlink"`
	UserUplinkPackets     *badjson.TypedMap[string, int64]  `json:"user_uplink_packets"`
	UserDownlinkPackets   *badjson.TypedMap[string, int64]  `json:"user_downlink_packets"`
	UserTCPSessions       *badjson.TypedMap[string, int64]  `json:"user_tcp_sessions"`
	UserUDPSessions       *badjson.TypedMap[string, int64]  `json:"user_udp_sessions"`
	Users                 *badjson.TypedMap[string, string] `json:"users"`
}

func (s *Service) loadCache() error {
	if s.cachePath == "" {
		return nil
	}
	basePath := filemanager.BasePath(s.ctx, s.cachePath)
	cacheBinary, err := os.ReadFile(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	err = s.decodeCache(cacheBinary)
	if err != nil {
		os.RemoveAll(basePath)
		return err
	}
	return nil
}

func (s *Service) saveCache() error {
	if s.cachePath == "" {
		return nil
	}
	basePath := filemanager.BasePath(s.ctx, s.cachePath)
	err := os.MkdirAll(filepath.Dir(basePath), 0o777)
	if err != nil {
		return err
	}
	cacheBinary, err := s.encodeCache()
	if err != nil {
		return err
	}
	return os.WriteFile(s.cachePath, cacheBinary, 0o644)
}

func (s *Service) decodeCache(cacheBinary []byte) error {
	if len(cacheBinary) == 0 {
		return nil
	}
	cache, err := json.UnmarshalExtended[*Cache](cacheBinary)
	if err != nil {
		return err
	}
	if cache.Endpoints == nil || cache.Endpoints.Size() == 0 {
		return nil
	}
	for _, entry := range cache.Endpoints.Entries() {
		trafficManager, loaded := s.traffics[entry.Key]
		if !loaded {
			continue
		}
		trafficManager.globalUplink.Store(entry.Value.GlobalUplink)
		trafficManager.globalDownlink.Store(entry.Value.GlobalDownlink)
		trafficManager.globalUplinkPackets.Store(entry.Value.GlobalUplinkPackets)
		trafficManager.globalDownlinkPackets.Store(entry.Value.GlobalDownlinkPackets)
		trafficManager.globalTCPSessions.Store(entry.Value.GlobalTCPSessions)
		trafficManager.globalUDPSessions.Store(entry.Value.GlobalUDPSessions)
		trafficManager.userUplink = typedAtomicInt64Map(entry.Value.UserUplink)
		trafficManager.userDownlink = typedAtomicInt64Map(entry.Value.UserDownlink)
		trafficManager.userUplinkPackets = typedAtomicInt64Map(entry.Value.UserUplinkPackets)
		trafficManager.userDownlinkPackets = typedAtomicInt64Map(entry.Value.UserDownlinkPackets)
		trafficManager.userTCPSessions = typedAtomicInt64Map(entry.Value.UserTCPSessions)
		trafficManager.userUDPSessions = typedAtomicInt64Map(entry.Value.UserUDPSessions)
		userManager, loaded := s.users[entry.Key]
		if !loaded {
			continue
		}
		userManager.usersMap = typedMap(entry.Value.Users)
		_ = userManager.postUpdate(false)
	}
	return nil
}

func (s *Service) encodeCache() ([]byte, error) {
	endpoints := new(badjson.TypedMap[string, *EndpointCache])
	for tag, traffic := range s.traffics {
		var (
			userUplink          = new(badjson.TypedMap[string, int64])
			userDownlink        = new(badjson.TypedMap[string, int64])
			userUplinkPackets   = new(badjson.TypedMap[string, int64])
			userDownlinkPackets = new(badjson.TypedMap[string, int64])
			userTCPSessions     = new(badjson.TypedMap[string, int64])
			userUDPSessions     = new(badjson.TypedMap[string, int64])
			userMap             = new(badjson.TypedMap[string, string])
		)
		for user, uplink := range traffic.userUplink {
			if uplink.Load() > 0 {
				userUplink.Put(user, uplink.Load())
			}
		}
		for user, downlink := range traffic.userDownlink {
			if downlink.Load() > 0 {
				userDownlink.Put(user, downlink.Load())
			}
		}
		for user, uplinkPackets := range traffic.userUplinkPackets {
			if uplinkPackets.Load() > 0 {
				userUplinkPackets.Put(user, uplinkPackets.Load())
			}
		}
		for user, downlinkPackets := range traffic.userDownlinkPackets {
			if downlinkPackets.Load() > 0 {
				userDownlinkPackets.Put(user, downlinkPackets.Load())
			}
		}
		for user, tcpSessions := range traffic.userTCPSessions {
			if tcpSessions.Load() > 0 {
				userTCPSessions.Put(user, tcpSessions.Load())
			}
		}
		for user, udpSessions := range traffic.userUDPSessions {
			if udpSessions.Load() > 0 {
				userUDPSessions.Put(user, udpSessions.Load())
			}
		}
		userManager := s.users[tag]
		if userManager != nil && len(userManager.usersMap) > 0 {
			userMap = new(badjson.TypedMap[string, string])
			for username, password := range userManager.usersMap {
				if username != "" && password != "" {
					userMap.Put(username, password)
				}
			}
		}
		endpoints.Put(tag, &EndpointCache{
			GlobalUplink:          traffic.globalUplink.Load(),
			GlobalDownlink:        traffic.globalDownlink.Load(),
			GlobalUplinkPackets:   traffic.globalUplinkPackets.Load(),
			GlobalDownlinkPackets: traffic.globalDownlinkPackets.Load(),
			GlobalTCPSessions:     traffic.globalTCPSessions.Load(),
			GlobalUDPSessions:     traffic.globalUDPSessions.Load(),
			UserUplink:            sortTypedMap(userUplink),
			UserDownlink:          sortTypedMap(userDownlink),
			UserUplinkPackets:     sortTypedMap(userUplinkPackets),
			UserDownlinkPackets:   sortTypedMap(userDownlinkPackets),
			UserTCPSessions:       sortTypedMap(userTCPSessions),
			UserUDPSessions:       sortTypedMap(userUDPSessions),
			Users:                 sortTypedMap(userMap),
		})
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(&Cache{
		Endpoints: sortTypedMap(endpoints),
	})
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sortTypedMap[T comparable](trafficMap *badjson.TypedMap[string, T]) *badjson.TypedMap[string, T] {
	if trafficMap == nil {
		return nil
	}
	keys := trafficMap.Keys()
	sort.Strings(keys)
	sortedMap := new(badjson.TypedMap[string, T])
	for _, key := range keys {
		value, _ := trafficMap.Get(key)
		sortedMap.Put(key, value)
	}
	return sortedMap
}

func typedAtomicInt64Map(trafficMap *badjson.TypedMap[string, int64]) map[string]*atomic.Int64 {
	result := make(map[string]*atomic.Int64)
	if trafficMap != nil {
		for _, entry := range trafficMap.Entries() {
			counter := new(atomic.Int64)
			counter.Store(entry.Value)
			result[entry.Key] = counter
		}
	}
	return result
}

func typedMap[T comparable](trafficMap *badjson.TypedMap[string, T]) map[string]T {
	result := make(map[string]T)
	if trafficMap != nil {
		for _, entry := range trafficMap.Entries() {
			result[entry.Key] = entry.Value
		}
	}
	return result
}
