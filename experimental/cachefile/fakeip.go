package cachefile

import (
	"net/netip"
	"os"

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
	err := c.batch(func(tx *bbolt.Tx) error {
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
	c.FakeIPSaveMetadataAsync(metadata)
	c.Flush()
	return nil
}

func (c *CacheFile) FakeIPSaveMetadataAsync(metadata *adapter.FakeIPMetadata) {
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	added := c.pending.fakeIPMetadata == nil
	c.pending.fakeIPMetadata = metadata
	c.enqueueLocked(added, 0)
}

func putFakeIPMetadata(tx *bbolt.Tx, metadata *adapter.FakeIPMetadata) error {
	bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
	if err != nil {
		return err
	}
	metadataBinary, err := metadata.MarshalBinary()
	if err != nil {
		return err
	}
	return bucket.Put(keyMetadata, metadataBinary)
}

func (c *CacheFile) FakeIPStore(address netip.Addr, domain string) error {
	c.queueFakeIP(address, domain)
	c.Flush()
	return nil
}

func (c *CacheFile) FakeIPStoreAsync(address netip.Addr, domain string, logger logger.Logger) {
	c.queueFakeIP(address, domain)
}

func (c *CacheFile) queueFakeIP(address netip.Addr, domain string) {
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	oldDomain, loaded := c.pending.fakeIPDomain[address]
	if loaded {
		if address.Is4() {
			delete(c.pending.fakeIPAddress4, oldDomain)
		} else {
			delete(c.pending.fakeIPAddress6, oldDomain)
		}
	}
	c.pending.fakeIPDomain[address] = domain
	if address.Is4() {
		c.pending.fakeIPAddress4[domain] = address
	} else {
		c.pending.fakeIPAddress6[domain] = address
	}
	c.enqueueLocked(!loaded, len(domain)-len(oldDomain))
}

func putFakeIP(tx *bbolt.Tx, address netip.Addr, domain string) error {
	bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
	if err != nil {
		return err
	}
	addressBytes := address.AsSlice()
	oldDomain := bucket.Get(addressBytes)
	err = bucket.Put(addressBytes, []byte(domain))
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
	if oldDomain != nil {
		err = bucket.Delete(oldDomain)
		if err != nil {
			return err
		}
	}
	return bucket.Put([]byte(domain), addressBytes)
}

func (c *CacheFile) FakeIPLoad(address netip.Addr) (string, bool) {
	c.pendingAccess.RLock()
	domain, cached := c.pending.fakeIPDomain[address]
	if !cached && c.writing != nil {
		domain, cached = c.writing.fakeIPDomain[address]
	}
	c.pendingAccess.RUnlock()
	if cached {
		return domain, true
	}
	_ = c.view(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketFakeIP)
		if bucket == nil {
			return nil
		}
		domain = string(bucket.Get(address.AsSlice()))
		return nil
	})
	return domain, domain != ""
}

func (p *pendingWrites) fakeIPAddress(domain string, isIPv6 bool) (netip.Addr, bool) {
	if isIPv6 {
		address, loaded := p.fakeIPAddress6[domain]
		return address, loaded
	}
	address, loaded := p.fakeIPAddress4[domain]
	return address, loaded
}

func (c *CacheFile) FakeIPLoadDomain(domain string, isIPv6 bool) (netip.Addr, bool) {
	c.pendingAccess.RLock()
	address, cached := c.pending.fakeIPAddress(domain, isIPv6)
	if !cached && c.writing != nil {
		address, cached = c.writing.fakeIPAddress(domain, isIPv6)
	}
	c.pendingAccess.RUnlock()
	if cached {
		return address, true
	}
	_ = c.view(func(tx *bbolt.Tx) error {
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
	c.flushAccess.Lock()
	defer c.flushAccess.Unlock()
	c.pendingAccess.Lock()
	for _, domain := range c.pending.fakeIPDomain {
		c.pending.count--
		c.pending.size -= len(domain)
	}
	clear(c.pending.fakeIPDomain)
	clear(c.pending.fakeIPAddress4)
	clear(c.pending.fakeIPAddress6)
	if c.pending.fakeIPMetadata != nil {
		c.pending.fakeIPMetadata = nil
		c.pending.count--
	}
	c.pendingAccess.Unlock()
	return c.batch(func(tx *bbolt.Tx) error {
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
