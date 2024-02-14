package cachefile

import (
	"encoding/binary"
	"time"

	"github.com/sagernet/bbolt"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/logger"
)

var bucketRDRC = []byte("rdrc")

func (c *CacheFile) StoreRDRC() bool {
	return c.storeRDRC
}

func (c *CacheFile) RDRCTimeout() time.Duration {
	return c.rdrcTimeout
}

func (c *CacheFile) LoadRDRC(transportName string, qName string) (rejected bool) {
	c.saveRDRCAccess.RLock()
	rejected, cached := c.saveRDRC[saveRDRCCacheKey{transportName, qName}]
	c.saveRDRCAccess.RUnlock()
	if cached {
		return
	}
	var deleteCache bool
	err := c.DB.View(func(tx *bbolt.Tx) error {
		bucket := c.bucket(tx, bucketRDRC)
		if bucket == nil {
			return nil
		}
		bucket = bucket.Bucket([]byte(transportName))
		if bucket == nil {
			return nil
		}
		content := bucket.Get([]byte(qName))
		if content == nil {
			return nil
		}
		expiresAt := time.Unix(int64(binary.BigEndian.Uint64(content)), 0)
		if time.Now().After(expiresAt) {
			deleteCache = true
			return nil
		}
		rejected = true
		return nil
	})
	if err != nil {
		return
	}
	if deleteCache {
		c.DB.Update(func(tx *bbolt.Tx) error {
			bucket := c.bucket(tx, bucketRDRC)
			if bucket == nil {
				return nil
			}
			bucket = bucket.Bucket([]byte(transportName))
			if bucket == nil {
				return nil
			}
			return bucket.Delete([]byte(qName))
		})
	}
	return
}

func (c *CacheFile) SaveRDRC(transportName string, qName string) error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		bucket, err := c.createBucket(tx, bucketRDRC)
		if err != nil {
			return err
		}
		bucket, err = bucket.CreateBucketIfNotExists([]byte(transportName))
		if err != nil {
			return err
		}
		expiresAt := buf.Get(8)
		defer buf.Put(expiresAt)
		binary.BigEndian.PutUint64(expiresAt, uint64(time.Now().Add(c.rdrcTimeout).Unix()))
		return bucket.Put([]byte(qName), expiresAt)
	})
}

func (c *CacheFile) SaveRDRCAsync(transportName string, qName string, logger logger.Logger) {
	saveKey := saveRDRCCacheKey{transportName, qName}
	c.saveRDRCAccess.Lock()
	c.saveRDRC[saveKey] = true
	c.saveRDRCAccess.Unlock()
	go func() {
		err := c.SaveRDRC(transportName, qName)
		if err != nil {
			logger.Warn("save RDRC: ", err)
		}
		c.saveRDRCAccess.Lock()
		delete(c.saveRDRC, saveKey)
		c.saveRDRCAccess.Unlock()
	}()
}
