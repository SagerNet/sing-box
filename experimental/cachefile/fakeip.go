package cachefile

import (
	"net/netip"
	"os"
	"time"

	"github.com/sagernet/bbolt"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
)

const fakeipBucketPrefix = "fakeip_"

var (
	bucketFakeIP        = []byte(fakeipBucketPrefix + "address")
	bucketFakeIPDomain4 = []byte(fakeipBucketPrefix + "domain4")
	bucketFakeIPDomain6 = []byte(fakeipBucketPrefix + "domain6")
	keyMetadata         = []byte(fakeipBucketPrefix + "metadata")
)

func (c *CacheFile) FakeIPMetadata() *adapter.FakeIPMetadata {
	var metadata adapter.FakeIPMetadata
	err := c.DB.Batch(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketFakeIP)
		if bucket == nil {
			return os.ErrNotExist
		}
		metadataBinary := bucket.Get(keyMetadata)
		if len(metadataBinary) == 0 {
			return os.ErrInvalid
		}
		err := bucket.Delete(keyMetadata)
		if err != nil {
			return err
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

func (c *CacheFile) FakeIPSaveMetadataAsync(metadata *adapter.FakeIPMetadata) {
	if timer := c.saveMetadataTimer; timer != nil {
		timer.Stop()
	}
	c.saveMetadataTimer = time.AfterFunc(10*time.Second, func() {
		_ = c.FakeIPSaveMetadata(metadata)
	})
}

func (c *CacheFile) FakeIPStore(address netip.Addr, domain string) error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
		if err != nil {
			return err
		}
		err = bucket.Put(address.AsSlice(), []byte(domain))
		if err != nil {
			return err
		}
		if address.Is4() {
			bucket, err = tx.CreateBucketIfNotExists(bucketFakeIPDomain4)
		} else {
			bucket, err = tx.CreateBucketIfNotExists(bucketFakeIPDomain6)
		}
		if err != nil {
			return err
		}
		return bucket.Put([]byte(domain), address.AsSlice())
	})
}

func (c *CacheFile) FakeIPStoreAsync(address netip.Addr, domain string, logger logger.Logger) {
	c.saveAccess.Lock()
	c.saveDomain[address] = domain
	if address.Is4() {
		c.saveAddress4[domain] = address
	} else {
		c.saveAddress6[domain] = address
	}
	c.saveAccess.Unlock()
	go func() {
		err := c.FakeIPStore(address, domain)
		if err != nil {
			logger.Warn("save FakeIP address pair: ", err)
		}
		c.saveAccess.Lock()
		delete(c.saveDomain, address)
		if address.Is4() {
			delete(c.saveAddress4, domain)
		} else {
			delete(c.saveAddress6, domain)
		}
		c.saveAccess.Unlock()
	}()
}

func (c *CacheFile) FakeIPLoad(address netip.Addr) (string, bool) {
	c.saveAccess.RLock()
	cachedDomain, cached := c.saveDomain[address]
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

func (c *CacheFile) FakeIPLoadDomain(domain string, isIPv6 bool) (netip.Addr, bool) {
	var (
		cachedAddress netip.Addr
		cached        bool
	)
	c.saveAccess.RLock()
	if !isIPv6 {
		cachedAddress, cached = c.saveAddress4[domain]
	} else {
		cachedAddress, cached = c.saveAddress6[domain]
	}
	c.saveAccess.RUnlock()
	if cached {
		return cachedAddress, true
	}
	var address netip.Addr
	_ = c.DB.View(func(tx *bbolt.Tx) error {
		var bucket *bbolt.Bucket
		if isIPv6 {
			bucket = tx.Bucket(bucketFakeIPDomain6)
		} else {
			bucket = tx.Bucket(bucketFakeIPDomain4)
		}
		if bucket == nil {
			return nil
		}
		address = M.AddrFromIP(bucket.Get([]byte(domain)))
		return nil
	})
	return address, address.IsValid()
}

func (c *CacheFile) FakeIPReset() error {
	return c.DB.Batch(func(tx *bbolt.Tx) error {
		err := tx.DeleteBucket(bucketFakeIP)
		if err != nil {
			return err
		}
		err = tx.DeleteBucket(bucketFakeIPDomain4)
		if err != nil {
			return err
		}
		return tx.DeleteBucket(bucketFakeIPDomain6)
	})
}
