package cachefile

import (
	"encoding/binary"
	"time"

	"github.com/sagernet/bbolt"
	"github.com/sagernet/sing/common/logger"
)

var bucketRDRC = []byte("rdrc2")

func (c *CacheFile) StoreRDRC() bool {
	return c.storeRDRC
}

func (c *CacheFile) RDRCTimeout() time.Duration {
	return c.rdrcTimeout
}

func (c *CacheFile) LoadRDRC(transportName string, qName string, qType uint16) (rejected bool) {
	key := saveCacheKey{transportName, qName, qType}
	now := time.Now()
	c.pendingAccess.RLock()
	expiresAt, loaded := c.pending.rdrc[key]
	if !loaded && c.writing != nil {
		expiresAt, loaded = c.writing.rdrc[key]
	}
	c.pendingAccess.RUnlock()
	if loaded && now.Before(expiresAt) {
		return true
	}
	err := c.view(func(tx *bbolt.Tx) error {
		bucket := c.bucket(tx, bucketRDRC)
		if bucket == nil {
			return nil
		}
		content := getCacheEntry(bucket, key)
		if len(content) < 8 {
			return nil
		}
		rejected = now.Before(time.Unix(int64(binary.BigEndian.Uint64(content)), 0))
		return nil
	})
	if err != nil {
		return false
	}
	return
}

func (c *CacheFile) SaveRDRC(transportName string, qName string, qType uint16) error {
	c.queueRDRC(transportName, qName, qType)
	c.Flush()
	return nil
}

func (c *CacheFile) SaveRDRCAsync(transportName string, qName string, qType uint16, logger logger.Logger) {
	c.queueRDRC(transportName, qName, qType)
}

func (c *CacheFile) queueRDRC(transportName string, qName string, qType uint16) {
	key := saveCacheKey{transportName, qName, qType}
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	_, loaded := c.pending.rdrc[key]
	c.pending.rdrc[key] = time.Now().Add(c.rdrcTimeout)
	if loaded {
		return
	}
	c.enqueueLocked(true, len(qName))
}
