package cachefile

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/bbolt"
	bboltErrors "github.com/sagernet/bbolt/errors"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service/filemanager"
)

var (
	bucketSelected = []byte("selected")
	bucketExpand   = []byte("group_expand")
	bucketMode     = []byte("clash_mode")
	bucketRuleSet  = []byte("rule_set")

	bucketNameList = []string{
		string(bucketSelected),
		string(bucketExpand),
		string(bucketMode),
		string(bucketRuleSet),
	}

	cacheIDDefault = []byte("default")
)

var _ adapter.CacheFile = (*CacheFile)(nil)

type CacheFile struct {
	ctx         context.Context
	path        string
	cacheID     []byte
	storeFakeIP bool

	DB                *bbolt.DB
	saveAccess        sync.RWMutex
	saveDomain        map[netip.Addr]string
	saveAddress4      map[string]netip.Addr
	saveAddress6      map[string]netip.Addr
	saveMetadataTimer *time.Timer
}

func NewCacheFile(ctx context.Context, options option.CacheFileOptions) *CacheFile {
	var path string
	if options.Path != "" {
		path = options.Path
	} else {
		path = "cache.db"
	}
	var cacheIDBytes []byte
	if options.CacheID != "" {
		cacheIDBytes = append([]byte{0}, []byte(options.CacheID)...)
	}
	return &CacheFile{
		ctx:          ctx,
		path:         filemanager.BasePath(ctx, path),
		cacheID:      cacheIDBytes,
		storeFakeIP:  options.StoreFakeIP,
		saveDomain:   make(map[netip.Addr]string),
		saveAddress4: make(map[string]netip.Addr),
		saveAddress6: make(map[string]netip.Addr),
	}
}

func (c *CacheFile) start() error {
	const fileMode = 0o666
	options := bbolt.Options{Timeout: time.Second}
	var (
		db  *bbolt.DB
		err error
	)
	for i := 0; i < 10; i++ {
		db, err = bbolt.Open(c.path, fileMode, &options)
		if err == nil {
			break
		}
		if errors.Is(err, bboltErrors.ErrTimeout) {
			continue
		}
		if E.IsMulti(err, bboltErrors.ErrInvalid, bboltErrors.ErrChecksum, bboltErrors.ErrVersionMismatch) {
			rmErr := os.Remove(c.path)
			if rmErr != nil {
				return err
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	err = filemanager.Chown(c.ctx, c.path)
	if err != nil {
		db.Close()
		return E.Cause(err, "platform chown")
	}
	err = db.Batch(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			if name[0] == 0 {
				return b.ForEachBucket(func(k []byte) error {
					bucketName := string(k)
					if !(common.Contains(bucketNameList, bucketName)) {
						_ = b.DeleteBucket(name)
					}
					return nil
				})
			} else {
				bucketName := string(name)
				if !(common.Contains(bucketNameList, bucketName) || strings.HasPrefix(bucketName, fakeipBucketPrefix)) {
					_ = tx.DeleteBucket(name)
				}
			}
			return nil
		})
	})
	if err != nil {
		db.Close()
		return err
	}
	c.DB = db
	return nil
}

func (c *CacheFile) PreStart() error {
	return c.start()
}

func (c *CacheFile) Start() error {
	return nil
}

func (c *CacheFile) Close() error {
	if c.DB == nil {
		return nil
	}
	return c.DB.Close()
}

func (c *CacheFile) StoreFakeIP() bool {
	return c.storeFakeIP
}

func (c *CacheFile) LoadMode() string {
	var mode string
	c.DB.View(func(t *bbolt.Tx) error {
		bucket := t.Bucket(bucketMode)
		if bucket == nil {
			return nil
		}
		var modeBytes []byte
		if len(c.cacheID) > 0 {
			modeBytes = bucket.Get(c.cacheID)
		} else {
			modeBytes = bucket.Get(cacheIDDefault)
		}
		mode = string(modeBytes)
		return nil
	})
	return mode
}

func (c *CacheFile) StoreMode(mode string) error {
	return c.DB.Batch(func(t *bbolt.Tx) error {
		bucket, err := t.CreateBucketIfNotExists(bucketMode)
		if err != nil {
			return err
		}
		if len(c.cacheID) > 0 {
			return bucket.Put(c.cacheID, []byte(mode))
		} else {
			return bucket.Put(cacheIDDefault, []byte(mode))
		}
	})
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

func (c *CacheFile) LoadGroupExpand(group string) (isExpand bool, loaded bool) {
	c.DB.View(func(t *bbolt.Tx) error {
		bucket := c.bucket(t, bucketExpand)
		if bucket == nil {
			return nil
		}
		expandBytes := bucket.Get([]byte(group))
		if len(expandBytes) == 1 {
			isExpand = expandBytes[0] == 1
			loaded = true
		}
		return nil
	})
	return
}

func (c *CacheFile) StoreGroupExpand(group string, isExpand bool) error {
	return c.DB.Batch(func(t *bbolt.Tx) error {
		bucket, err := c.createBucket(t, bucketExpand)
		if err != nil {
			return err
		}
		if isExpand {
			return bucket.Put([]byte(group), []byte{1})
		} else {
			return bucket.Put([]byte(group), []byte{0})
		}
	})
}

func (c *CacheFile) LoadRuleSet(tag string) *adapter.SavedRuleSet {
	var savedSet adapter.SavedRuleSet
	err := c.DB.View(func(t *bbolt.Tx) error {
		bucket := c.bucket(t, bucketRuleSet)
		if bucket == nil {
			return os.ErrNotExist
		}
		setBinary := bucket.Get([]byte(tag))
		if len(setBinary) == 0 {
			return os.ErrInvalid
		}
		return savedSet.UnmarshalBinary(setBinary)
	})
	if err != nil {
		return nil
	}
	return &savedSet
}

func (c *CacheFile) SaveRuleSet(tag string, set *adapter.SavedRuleSet) error {
	return c.DB.Batch(func(t *bbolt.Tx) error {
		bucket, err := c.createBucket(t, bucketRuleSet)
		if err != nil {
			return err
		}
		setBinary, err := set.MarshalBinary()
		if err != nil {
			return err
		}
		return bucket.Put([]byte(tag), setBinary)
	})
}
