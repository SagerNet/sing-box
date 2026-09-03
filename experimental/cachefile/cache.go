package cachefile

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/bbolt"
	bboltErrors "github.com/sagernet/bbolt/errors"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
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
		string(bucketRDRC),
		string(bucketDNSCache),
	}

	cacheIDDefault = []byte("default")

	databaseOptions = bbolt.Options{
		Timeout:        time.Second,
		NoFreelistSync: true,
	}
)

var _ adapter.CacheFile = (*CacheFile)(nil)

type CacheFile struct {
	ctx               context.Context
	logger            logger.Logger
	path              string
	cacheID           []byte
	cacheIDText       string
	storeFakeIP       bool
	storeRDRC         bool
	storeDNS          bool
	disableExpire     bool
	rdrcTimeout       time.Duration
	optimisticTimeout time.Duration
	bufferSize        int
	flushInterval     time.Duration
	DB                *bbolt.DB
	dbAccess          sync.RWMutex
	pendingAccess     sync.RWMutex
	pending           *pendingWrites
	writing           *pendingWrites
	flushAccess       sync.Mutex
	flushTimer        *time.Timer
	flushSignal       chan struct{}
	done              chan struct{}
	closeOnce         sync.Once
}

type saveCacheKey struct {
	TransportName string
	QuestionName  string
	QType         uint16
}

type saveDNSCacheEntry struct {
	value []byte
}

func New(ctx context.Context, logger logger.Logger, options option.CacheFileOptions) *CacheFile {
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
	//nolint:staticcheck
	storeRDRC := options.StoreRDRC
	//nolint:staticcheck
	rdrcTimeout := time.Duration(options.RDRCTimeout)
	if storeRDRC {
		deprecated.Report(ctx, deprecated.OptionStoreRDRC)
		if rdrcTimeout <= 0 {
			rdrcTimeout = 7 * 24 * time.Hour
		}
	} else {
		rdrcTimeout = 0
	}
	bufferSize := int(options.BufferSize.Value())
	if bufferSize == 0 {
		bufferSize = defaultBufferSize
	}
	flushTimer := time.NewTimer(time.Hour)
	flushTimer.Stop()
	return &CacheFile{
		ctx:           ctx,
		logger:        logger,
		path:          filemanager.BasePath(ctx, path),
		cacheID:       cacheIDBytes,
		cacheIDText:   options.CacheID,
		storeFakeIP:   options.StoreFakeIP,
		storeRDRC:     storeRDRC,
		storeDNS:      options.StoreDNS,
		rdrcTimeout:   rdrcTimeout,
		bufferSize:    bufferSize,
		flushInterval: time.Duration(options.FlushInterval),
		pending:       newPendingWrites(),
		flushTimer:    flushTimer,
		flushSignal:   make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
}

func (c *CacheFile) Name() string {
	return "cache-file"
}

func (c *CacheFile) Dependencies() []string {
	return nil
}

func (c *CacheFile) CacheID() string {
	return c.cacheIDText
}

func (c *CacheFile) SetOptimisticTimeout(timeout time.Duration) {
	c.optimisticTimeout = timeout
}

func (c *CacheFile) SetDisableExpire(disableExpire bool) {
	c.disableExpire = disableExpire
}

func (c *CacheFile) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateInitialize:
		return c.start()
	case adapter.StartStateStart:
		c.startCacheCleanup()
	}
	return nil
}

func (c *CacheFile) startCacheCleanup() {
	if c.storeDNS {
		c.clearRDRC()
		c.cleanupDNSCache()
		interval := c.optimisticTimeout / 2
		if interval <= 0 {
			interval = time.Hour
		}
		go c.loopCacheCleanup(interval, c.cleanupDNSCache)
	} else if c.storeRDRC {
		c.cleanupRDRC()
		interval := c.rdrcTimeout / 2
		if interval <= 0 {
			interval = time.Hour
		}
		go c.loopCacheCleanup(interval, c.cleanupRDRC)
	}
}

func (c *CacheFile) start() error {
	const fileMode = 0o666
	cacheFile, err := filemanager.OpenFile(c.ctx, c.path, os.O_RDWR|os.O_CREATE, fileMode)
	if err != nil {
		return err
	}
	cacheFile.Close()
	var db *bbolt.DB
	for range 10 {
		db, err = bbolt.Open(c.path, fileMode, &databaseOptions)
		if err != nil {
			if errors.Is(err, bboltErrors.ErrTimeout) {
				continue
			}
			if E.IsMulti(err, bboltErrors.ErrInvalid, bboltErrors.ErrChecksum, bboltErrors.ErrVersionMismatch) {
				rmErr := filemanager.Remove(c.ctx, c.path)
				if rmErr != nil {
					return err
				}
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		err = checkDatabase(db)
		if err == nil {
			break
		}
		c.logger.Error("database corrupted: ", err, ": resetting")
		db.Close()
		rmErr := filemanager.Remove(c.ctx, c.path)
		if rmErr != nil {
			return err
		}
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
					if !common.Contains(bucketNameList, bucketName) {
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
	go c.loopFlush()
	return nil
}

func (c *CacheFile) Close() error {
	c.closeOnce.Do(func() {
		close(c.done)
	})
	c.flushTimer.Stop()
	c.dbAccess.RLock()
	db := c.DB
	c.dbAccess.RUnlock()
	if db == nil {
		return nil
	}
	c.Flush()
	return db.Close()
}

func checkDatabase(db *bbolt.DB) error {
	return db.View(func(tx *bbolt.Tx) error {
		var checkErr error
		for txErr := range tx.Check() {
			if checkErr == nil {
				checkErr = txErr
			}
		}
		return checkErr
	})
}

func (c *CacheFile) database() *bbolt.DB {
	c.dbAccess.RLock()
	defer c.dbAccess.RUnlock()
	return c.DB
}

func (c *CacheFile) view(fn func(tx *bbolt.Tx) error) (err error) {
	db := c.database()
	defer func() {
		r := recover()
		if r != nil {
			c.resetDB(db, r)
			err = E.New("database corrupted: ", r)
		}
	}()
	return db.View(fn)
}

func (c *CacheFile) batch(fn func(tx *bbolt.Tx) error) (err error) {
	db := c.database()
	defer func() {
		r := recover()
		if r != nil {
			c.resetDB(db, r)
			err = E.New("database corrupted: ", r)
		}
	}()
	err = db.Batch(fn)
	var panicErr bbolt.PanickedError
	if errors.As(err, &panicErr) {
		c.resetDB(db, panicErr.Reason)
		return E.New("database corrupted: ", panicErr.Reason)
	}
	return err
}

func (c *CacheFile) update(fn func(tx *bbolt.Tx) error) (err error) {
	db := c.database()
	defer func() {
		r := recover()
		if r != nil {
			c.resetDB(db, r)
			err = E.New("database corrupted: ", r)
		}
	}()
	return db.Update(fn)
}

func (c *CacheFile) resetDB(failedDB *bbolt.DB, reason any) {
	c.dbAccess.Lock()
	defer c.dbAccess.Unlock()
	if c.DB != failedDB {
		return
	}
	c.logger.Error("database corrupted: ", reason, ": resetting")
	failedDB.Close()
	filemanager.Remove(c.ctx, c.path)
	db, err := bbolt.Open(c.path, 0o666, &databaseOptions)
	if err == nil {
		_ = filemanager.Chown(c.ctx, c.path)
		c.DB = db
	}
}

func (c *CacheFile) StoreFakeIP() bool {
	return c.storeFakeIP
}

func (c *CacheFile) LoadMode() string {
	var mode string
	c.view(func(t *bbolt.Tx) error {
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
	return c.batch(func(t *bbolt.Tx) error {
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
	c.view(func(t *bbolt.Tx) error {
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
	return c.batch(func(t *bbolt.Tx) error {
		bucket, err := c.createBucket(t, bucketSelected)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(group), []byte(selected))
	})
}

func (c *CacheFile) LoadGroupExpand(group string) (isExpand bool, loaded bool) {
	c.view(func(t *bbolt.Tx) error {
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
	return c.batch(func(t *bbolt.Tx) error {
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

func (c *CacheFile) LoadRuleSet(tag string) *adapter.SavedBinary {
	var savedSet adapter.SavedBinary
	err := c.view(func(t *bbolt.Tx) error {
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

func (c *CacheFile) SaveRuleSet(tag string, set *adapter.SavedBinary) error {
	return c.batch(func(t *bbolt.Tx) error {
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
