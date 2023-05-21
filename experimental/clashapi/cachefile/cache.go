package cachefile

import (
	"os"
	"time"

	"github.com/sagernet/sing-box/adapter"

	"go.etcd.io/bbolt"
)

var bucketSelected = []byte("selected")

var _ adapter.ClashCacheFile = (*CacheFile)(nil)

type CacheFile struct {
	DB      *bbolt.DB
	cacheID []byte
}

func Open(path string, cacheID string) (*CacheFile, error) {
	const fileMode = 0o666
	options := bbolt.Options{Timeout: time.Second}
	db, err := bbolt.Open(path, fileMode, &options)
	switch err {
	case bbolt.ErrInvalid, bbolt.ErrChecksum, bbolt.ErrVersionMismatch:
		if err = os.Remove(path); err != nil {
			break
		}
		db, err = bbolt.Open(path, 0o666, &options)
	}
	if err != nil {
		return nil, err
	}
	var cacheIDBytes []byte
	if cacheID != "" {
		cacheIDBytes = append([]byte{0}, []byte(cacheID)...)
	}
	return &CacheFile{db, cacheIDBytes}, nil
}

func (c *CacheFile) bucket(t *bbolt.Tx, key []byte) *bbolt.Bucket {
	if c.cacheID == nil {
		return t.Bucket(key)
	}
	bucket := t.Bucket(c.cacheID)
	if bucket == nil {
		return nil
	}
	return bucket.Bucket(key)
}

func (c *CacheFile) createBucket(t *bbolt.Tx, key []byte) (*bbolt.Bucket, error) {
	if c.cacheID == nil {
		return t.CreateBucketIfNotExists(key)
	}
	bucket, err := t.CreateBucketIfNotExists(c.cacheID)
	if bucket == nil {
		return nil, err
	}
	return bucket.CreateBucketIfNotExists(key)
}

func (c *CacheFile) LoadSelected(group string) string {
	var selected string
	c.DB.View(func(t *bbolt.Tx) error {
		bucket := c.bucket(t, bucketSelected)
		if bucket == nil {
			return nil
		}
		selectedBytes := bucket.Get([]byte(group))
		if len(selectedBytes) > 0 {
			selected = string(selectedBytes)
		}
		return nil
	})
	return selected
}

func (c *CacheFile) StoreSelected(group, selected string) error {
	return c.DB.Batch(func(t *bbolt.Tx) error {
		bucket, err := c.createBucket(t, bucketSelected)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(group), []byte(selected))
	})
}

func (c *CacheFile) Close() error {
	return c.DB.Close()
}
