package gocache

import (
	"sync"
	"sync/atomic"
	"time"
)

type Cacher interface {
	Start()
	Get(key interface{}) interface{}
	Store(key interface{}, value interface{}, duration time.Duration) bool
	Stop()
}

// gocache factory method
// parameters
// - shortKeyString func(key interface{}) uint : generating function index of buckets
// - keyString func(key interface{}) string : generating function key of cache as a string
// - refreshDuration time.Duration : refresh duration of caches remove too old to keep(ex. this is time.Second, gocache calls gocache.refresh() every seconds)
// - size int : limit number of cached instance(ex. size = 100, gocache contains 100 cached item as a maximum)
// - bucketSize int : a number of separated caches (to avoid lock) (ex. bucketSize is 10, drop down the probability of locking to 1/10)
func New(shortKeyString func(key interface{}) uint, keyString func(key interface{}) string, refreshDuration time.Duration, size int, bucketSize int) Cacher {
	ret := &bucketCache{
		caches:          make([]BaseCache, bucketSize, bucketSize),
		size:            int32(size),
		bucketSize:      bucketSize,
		capacity:        int32(size),
		shortKeyString:  shortKeyString,
		refreshDuration: refreshDuration,
		stop:            make(chan struct{}, 1),
	}

	for i := 0; i < ret.bucketSize; i++ {
		front := make(map[string]*item, size/ret.bucketSize)
		back := make(map[string]*item, size/ret.bucketSize)

		ret.caches[i] = BaseCache{
			keyString:     keyString,
			front:         &front,
			back:          &back,
			keyBufferSize: size / ret.bucketSize,
		}
	}

	return ret
}

type item struct {
	v    interface{}
	time time.Time
}

type bucketCache struct {
	caches         []BaseCache
	size           int32
	bucketSize     int
	capacity       int32
	shortKeyString func(key interface{}) uint
	stop           chan struct{}

	refreshDuration time.Duration
}

func (r *bucketCache) Start() {
	for {
		select {
		case <-r.stop:
			return
		case <-time.After(r.refreshDuration):
			var sum int32
			for i := 0; i < int(r.bucketSize); i++ {
				sum += r.caches[i].refresh()
			}

			atomic.StoreInt32(&r.capacity, r.size-sum)
		}
	}
}

func (r *bucketCache) Stop() {
	r.stop <- struct{}{}
}

func (r *bucketCache) Get(key interface{}) interface{} {
	idx := r.getBucketIndex(key)

	stored := r.caches[idx].get(key)

	if stored == nil {
		return nil
	}

	return stored.v
}

func (r *bucketCache) getBucketIndex(key interface{}) uint {
	return r.shortKeyString(key) % uint(r.bucketSize)
}

func (r *bucketCache) Store(key interface{}, value interface{}, duration time.Duration) bool {
	if atomic.LoadInt32(&r.capacity) <= 0 {
		return false
	}

	atomic.AddInt32(&r.capacity, -1)

	idx := r.getBucketIndex(key)

	delta := r.caches[idx].store(key, value, duration) - 1

	atomic.AddInt32(&r.capacity, delta)

	return delta == 0
}

type BaseCache struct {
	front         *map[string]*item            // map to read.
	back          *map[string]*item            // map to back write. write back -> swap with front -> write back again
	flock         sync.RWMutex                 // lock for front
	block         sync.RWMutex                 // lock for back
	keyString     func(key interface{}) string // make key string from request
	keyBufferSize int
}

func (r *BaseCache) get(key interface{}) *item {
	keyString := r.keyString(key)

	r.flock.RLock()
	defer func() {
		r.flock.RUnlock()
	}()
	return (*r.front)[keyString]
}

func (r *BaseCache) store(key interface{}, value interface{}, duration time.Duration) int32 {
	keyString := r.keyString(key)

	if r.get(key) != nil {
		return 0
	}

	stored := &item{
		v:    value,
		time: time.Now().Add(duration),
	}

	r.block.Lock()
	(*r.back)[keyString] = stored

	r.swap()

	(*r.back)[keyString] = stored
	r.block.Unlock()

	return 1
}

func (r *BaseCache) refresh() int32 {
	now := time.Now()

	keys := make([]string, 0, r.keyBufferSize)

	r.block.RLock()
	for k, v := range *r.back {
		if now.After(v.time) {
			keys = append(keys, k)
		}
	}
	r.block.RUnlock()

	r.block.Lock()
	for _, key := range keys {
		delete(*r.back, key)
	}

	r.swap()

	for _, key := range keys {
		delete(*r.back, key)
	}
	r.block.Unlock()

	return int32(len(*r.back))
}

func (r *BaseCache) swap() {
	r.flock.Lock()
	defer func() {
		r.flock.Unlock()
	}()

	temp := r.front
	r.front = r.back
	r.back = temp
}