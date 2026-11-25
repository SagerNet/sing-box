package cachefile

import (
	"bytes"
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
	err := c.view(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketFakeIP)
		if bucket == nil {
			return os.ErrNotExist
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
	return c.batch(func(tx *bbolt.Tx) error {
		return putFakeIPMetadata(tx, metadata)
	})
}

func (c *CacheFile) FakeIPReserve(metadata *adapter.FakeIPMetadata, current netip.Addr, reservedEnd netip.Addr) error {
	return c.batch(func(tx *bbolt.Tx) error {
		err := putFakeIPMetadata(tx, metadata)
		if err != nil {
			return err
		}
		return deleteFakeIPRange(tx, current, reservedEnd)
	})
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

func deleteFakeIPRange(tx *bbolt.Tx, current netip.Addr, reservedEnd netip.Addr) error {
	bucket := tx.Bucket(bucketFakeIP)
	if bucket == nil {
		return nil
	}
	var domainBucket *bbolt.Bucket
	if current.Is4() {
		domainBucket = tx.Bucket(bucketFakeIPDomain4)
	} else {
		domainBucket = tx.Bucket(bucketFakeIPDomain6)
	}
	from := current.AsSlice()
	to := reservedEnd.AsSlice()
	wrapped := bytes.Compare(to, from) <= 0
	var addresses, domains [][]byte
	cursor := bucket.Cursor()
	for key, value := cursor.Seek(from); key != nil; key, value = cursor.Next() {
		if !wrapped && bytes.Compare(key, to) > 0 {
			break
		}
		if len(key) == len(from) && !bytes.Equal(key, from) {
			addresses = append(addresses, key)
			domains = append(domains, value)
		}
	}
	if wrapped {
		for key, value := cursor.First(); key != nil && bytes.Compare(key, to) <= 0; key, value = cursor.Next() {
			if len(key) == len(from) {
				addresses = append(addresses, key)
				domains = append(domains, value)
			}
		}
	}
	for i, address := range addresses {
		err := bucket.Delete(address)
		if err != nil {
			return err
		}
		if domainBucket != nil && bytes.Equal(domainBucket.Get(domains[i]), address) {
			err = domainBucket.Delete(domains[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CacheFile) FakeIPStore(address netip.Addr, domain string) error {
	return c.batch(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketFakeIP)
		if err != nil {
			return err
		}
		oldDomain := bucket.Get(address.AsSlice())
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
		if oldDomain != nil && bytes.Equal(bucket.Get(oldDomain), address.AsSlice()) {
			err = bucket.Delete(oldDomain)
			if err != nil {
				return err
			}
		}
		return bucket.Put([]byte(domain), address.AsSlice())
	})
}

func (c *CacheFile) FakeIPStoreAsync(address netip.Addr, domain string, logger logger.Logger) {
	c.saveFakeIPAccess.Lock()
	if oldDomain, loaded := c.saveDomain[address]; loaded {
		if address.Is4() {
			delete(c.saveAddress4, oldDomain)
		} else {
			delete(c.saveAddress6, oldDomain)
		}
	}
	c.saveDomain[address] = domain
	if address.Is4() {
		c.saveAddress4[domain] = address
	} else {
		c.saveAddress6[domain] = address
	}
	c.saveFakeIPAccess.Unlock()
	go func() {
		err := c.FakeIPStore(address, domain)
		if err != nil {
			logger.Warn("save FakeIP cache: ", err)
		}
		c.saveFakeIPAccess.Lock()
		delete(c.saveDomain, address)
		if address.Is4() {
			delete(c.saveAddress4, domain)
		} else {
			delete(c.saveAddress6, domain)
		}
		c.saveFakeIPAccess.Unlock()
	}()
}

func (c *CacheFile) FakeIPLoad(address netip.Addr) (string, bool) {
	c.saveFakeIPAccess.RLock()
	cachedDomain, cached := c.saveDomain[address]
	c.saveFakeIPAccess.RUnlock()
	if cached {
		return cachedDomain, true
	}
	var domain string
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

func (c *CacheFile) FakeIPLoadDomain(domain string, isIPv6 bool) (netip.Addr, bool) {
	var (
		cachedAddress netip.Addr
		cached        bool
	)
	c.saveFakeIPAccess.RLock()
	if !isIPv6 {
		cachedAddress, cached = c.saveAddress4[domain]
	} else {
		cachedAddress, cached = c.saveAddress6[domain]
	}
	c.saveFakeIPAccess.RUnlock()
	if cached {
		return cachedAddress, true
	}
	var address netip.Addr
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
	return c.batch(func(tx *bbolt.Tx) error {
		for _, bucketName := range [][]byte{bucketFakeIP, bucketFakeIPDomain4, bucketFakeIPDomain6} {
			if tx.Bucket(bucketName) == nil {
				continue
			}
			err := tx.DeleteBucket(bucketName)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
