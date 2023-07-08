package cachefile

import (
	"net/netip"
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/logger"

	"go.etcd.io/bbolt"
)

var (
	bucketFakeIP = []byte("fakeip")
	keyMetadata  = []byte("metadata")
)

func (c *CacheFile) FakeIPMetadata() *adapter.FakeIPMetadata {
	var metadata adapter.FakeIPMetadata
	err := c.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketFakeIP)
		if bucket == nil {
			return nil
		}
		metadataBinary := bucket.Get(keyMetadata)
		if len(metadataBinary) == 0 {
			return os.ErrInvalid
		}
		return metadata.UnmarshalBinary(metadataBinary)
	})
	if err != nil {
		return nil
	}
	return &metadata
}

func (c *CacheFile) FakeIPSaveMetadata(metadata *adapter.FakeIPMetadata) error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
		if err != nil {
			return err
		}
		metadataBinary, err := metadata.MarshalBinary()
		if err != nil {
			return err
		}
		return bucket.Put(keyMetadata, metadataBinary)
	})
}

func (c *CacheFile) FakeIPStore(address netip.Addr, domain string) error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
		if err != nil {
			return err
		}
		return bucket.Put(address.AsSlice(), []byte(domain))
	})
}

func (c *CacheFile) FakeIPStoreAsync(address netip.Addr, domain string, logger logger.Logger) {
	c.saveAccess.Lock()
	c.saveCache[address] = domain
	c.saveAccess.Unlock()
	go func() {
		err := c.FakeIPStore(address, domain)
		if err != nil {
			logger.Warn("save FakeIP address pair: ", err)
		}
		c.saveAccess.Lock()
		delete(c.saveCache, address)
		c.saveAccess.Unlock()
	}()
}

func (c *CacheFile) FakeIPLoad(address netip.Addr) (string, bool) {
	c.saveAccess.RLock()
	cachedDomain, cached := c.saveCache[address]
	c.saveAccess.RUnlock()
	if cached {
		return cachedDomain, true
	}
	var domain string
	_ = c.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketFakeIP)
		if bucket == nil {
			return nil
		}
		domain = string(bucket.Get(address.AsSlice()))
		return nil
	})
	return domain, domain != ""
}

func (c *CacheFile) FakeIPReset() error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket(bucketFakeIP)
	})
}
