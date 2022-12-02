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
	DB *bbolt.DB
}

func Open(path string) (*CacheFile, error) {
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
	return &CacheFile{db}, nil
}

func (c *CacheFile) LoadSelected(group string) string {
	var selected string
	c.DB.View(func(t *bbolt.Tx) error {
		bucket := t.Bucket(bucketSelected)
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
		bucket, err := t.CreateBucketIfNotExists(bucketSelected)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(group), []byte(selected))
	})
}

func (c *CacheFile) Close() error {
	return c.DB.Close()
}
