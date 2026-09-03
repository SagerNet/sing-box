package cachefile

import (
	"encoding/binary"
	"time"

	"github.com/sagernet/bbolt"
	"github.com/sagernet/sing/common/logger"
)

var bucketDNSCache = []byte("dns_cache")

func (c *CacheFile) StoreDNS() bool {
	return c.storeDNS
}

func (c *CacheFile) LoadDNSCache(transportName string, qName string, qType uint16) (rawMessage []byte, expireAt time.Time, loaded bool) {
	key := saveCacheKey{transportName, qName, qType}
	c.pendingAccess.RLock()
	entry, cached := c.pending.dnsCache[key]
	if !cached && c.writing != nil {
		entry, cached = c.writing.dnsCache[key]
	}
	c.pendingAccess.RUnlock()
	if cached {
		return entry.value[8:], time.Unix(int64(binary.BigEndian.Uint64(entry.value[:8])), 0), true
	}
	err := c.view(func(tx *bbolt.Tx) error {
		bucket := c.bucket(tx, bucketDNSCache)
		if bucket == nil {
			return nil
		}
		content := getCacheEntry(bucket, key)
		if len(content) < 8 {
			return nil
		}
		expireAt = time.Unix(int64(binary.BigEndian.Uint64(content[:8])), 0)
		rawMessage = make([]byte, len(content)-8)
		copy(rawMessage, content[8:])
		loaded = true
		return nil
	})
	if err != nil {
		return nil, time.Time{}, false
	}
	return
}

func (c *CacheFile) SaveDNSCache(transportName string, qName string, qType uint16, rawMessage []byte, expireAt time.Time) error {
	c.queueDNSCache(transportName, qName, qType, rawMessage, expireAt)
	c.Flush()
	return nil
}

func (c *CacheFile) SaveDNSCacheAsync(transportName string, qName string, qType uint16, rawMessage []byte, expireAt time.Time, logger logger.Logger) {
	c.queueDNSCache(transportName, qName, qType, rawMessage, expireAt)
}

func (c *CacheFile) queueDNSCache(transportName string, qName string, qType uint16, rawMessage []byte, expireAt time.Time) {
	value := make([]byte, 8+len(rawMessage))
	binary.BigEndian.PutUint64(value[:8], uint64(expireAt.Unix()))
	copy(value[8:], rawMessage)
	key := saveCacheKey{transportName, qName, qType}
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	oldEntry, loaded := c.pending.dnsCache[key]
	c.pending.dnsCache[key] = saveDNSCacheEntry{value}
	c.enqueueLocked(!loaded, len(qName)+len(value)-len(oldEntry.value))
}

func (c *CacheFile) ClearDNSCache() error {
	c.flushAccess.Lock()
	defer c.flushAccess.Unlock()
	c.pendingAccess.Lock()
	for key, entry := range c.pending.dnsCache {
		c.pending.count--
		c.pending.size -= len(key.QuestionName) + len(entry.value)
	}
	clear(c.pending.dnsCache)
	c.pendingAccess.Unlock()
	return c.batch(func(tx *bbolt.Tx) error {
		if c.cacheID == nil {
			bucket := tx.Bucket(bucketDNSCache)
			if bucket == nil {
				return nil
			}
			return tx.DeleteBucket(bucketDNSCache)
		}
		bucket := tx.Bucket(c.cacheID)
		if bucket == nil || bucket.Bucket(bucketDNSCache) == nil {
			return nil
		}
		return bucket.DeleteBucket(bucketDNSCache)
	})
}

func (c *CacheFile) loopCacheCleanup(interval time.Duration, cleanupFunc func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			cleanupFunc()
		}
	}
}

func (c *CacheFile) cleanupDNSCache() {
	now := time.Now()
	err := c.batch(func(tx *bbolt.Tx) error {
		bucket := c.bucket(tx, bucketDNSCache)
		if bucket == nil {
			return nil
		}
		var emptyTransports [][]byte
		err := bucket.ForEachBucket(func(transportName []byte) error {
			transportBucket := bucket.Bucket(transportName)
			if transportBucket == nil {
				return nil
			}
			var expiredKeys [][]byte
			err := transportBucket.ForEach(func(key, value []byte) error {
				if len(value) < 8 {
					expiredKeys = append(expiredKeys, append([]byte(nil), key...))
					return nil
				}
				if c.disableExpire {
					return nil
				}
				expireAt := time.Unix(int64(binary.BigEndian.Uint64(value[:8])), 0)
				if now.After(expireAt.Add(c.optimisticTimeout)) {
					expiredKeys = append(expiredKeys, append([]byte(nil), key...))
				}
				return nil
			})
			if err != nil {
				return err
			}
			for _, key := range expiredKeys {
				err = transportBucket.Delete(key)
				if err != nil {
					return err
				}
			}
			first, _ := transportBucket.Cursor().First()
			if first == nil {
				emptyTransports = append(emptyTransports, append([]byte(nil), transportName...))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, name := range emptyTransports {
			err = bucket.DeleteBucket(name)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.logger.Warn("cleanup DNS cache: ", err)
	}
}

func (c *CacheFile) clearRDRC() {
	c.flushAccess.Lock()
	defer c.flushAccess.Unlock()
	c.pendingAccess.Lock()
	for key := range c.pending.rdrc {
		c.pending.count--
		c.pending.size -= len(key.QuestionName)
	}
	clear(c.pending.rdrc)
	c.pendingAccess.Unlock()
	err := c.batch(func(tx *bbolt.Tx) error {
		if c.cacheID == nil {
			if tx.Bucket(bucketRDRC) == nil {
				return nil
			}
			return tx.DeleteBucket(bucketRDRC)
		}
		bucket := tx.Bucket(c.cacheID)
		if bucket == nil || bucket.Bucket(bucketRDRC) == nil {
			return nil
		}
		return bucket.DeleteBucket(bucketRDRC)
	})
	if err != nil {
		c.logger.Warn("clear RDRC: ", err)
	}
}

func (c *CacheFile) cleanupRDRC() {
	now := time.Now()
	err := c.batch(func(tx *bbolt.Tx) error {
		bucket := c.bucket(tx, bucketRDRC)
		if bucket == nil {
			return nil
		}
		var emptyTransports [][]byte
		err := bucket.ForEachBucket(func(transportName []byte) error {
			transportBucket := bucket.Bucket(transportName)
			if transportBucket == nil {
				return nil
			}
			var expiredKeys [][]byte
			err := transportBucket.ForEach(func(key, value []byte) error {
				if len(value) < 8 {
					expiredKeys = append(expiredKeys, append([]byte(nil), key...))
					return nil
				}
				expiresAt := time.Unix(int64(binary.BigEndian.Uint64(value)), 0)
				if now.After(expiresAt) {
					expiredKeys = append(expiredKeys, append([]byte(nil), key...))
				}
				return nil
			})
			if err != nil {
				return err
			}
			for _, key := range expiredKeys {
				err = transportBucket.Delete(key)
				if err != nil {
					return err
				}
			}
			first, _ := transportBucket.Cursor().First()
			if first == nil {
				emptyTransports = append(emptyTransports, append([]byte(nil), transportName...))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, name := range emptyTransports {
			err = bucket.DeleteBucket(name)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.logger.Warn("cleanup RDRC: ", err)
	}
}
