package performance

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type singleBucketCache struct {
	caches         []insideCache
	size           int32
	bucketSize     int
	capacity       int32
	shortKeyString func(key interface{}) uint
	stop           chan struct{}

	refreshDuration time.Duration
	refreshCount    int
}

func (r *singleBucketCache) Start() {
	now := time.Now()
	for {
		select {
		case <-r.stop:
			return
		case <-time.After(r.refreshDuration):
			var sum int32
			for i := 0; i < r.bucketSize; i++ {
				sum += r.caches[i].refresh()
			}

			atomic.StoreInt32(&r.capacity, r.size-sum)
			r.refreshCount++
			oldNow := now
			now = time.Now()
			fmt.Println("singleBucketCache = ", now.Sub(oldNow))
		}
	}
}

func (r *singleBucketCache) Stop() {
	r.stop <- struct{}{}
}

func (r *singleBucketCache) Get(key interface{}) interface{} {
	keyString := r.shortKeyString(key) % uint(r.bucketSize)
	stored := r.caches[keyString].get(key)

	if stored == nil {
		return nil
	}

	return stored.v
}

func (r *singleBucketCache) Store(key interface{}, v interface{}, duration time.Duration) bool {
	if atomic.LoadInt32(&r.capacity) <= 0 {
		return false
	}

	atomic.AddInt32(&r.capacity, -1)

	keyString := r.shortKeyString(key) % uint(r.bucketSize)

	delta := r.caches[keyString].store(key, v, duration) - 1

	atomic.AddInt32(&r.capacity, delta)

	return delta == 0
}

func (r *singleBucketCache) remains() {
	for i := 0; i < len(r.caches); i++ {
		fmt.Println(len(r.caches[i].m))
	}
}

func (r *singleBucketCache) read() time.Duration {
	var ret time.Duration
	for i := 0; i < len(r.caches); i++ {
		ret += r.caches[i].read
	}
	return ret
}

func (r *singleBucketCache) write() time.Duration {
	var ret time.Duration
	for i := 0; i < len(r.caches); i++ {
		ret += r.caches[i].write
	}
	return ret
}

type insideCache struct {
	m             map[string]*item
	lock          sync.RWMutex
	keyString     func(key interface{}) string
	keyBufferSize int

	read  time.Duration
	write time.Duration
}

func (r *insideCache) get(key interface{}) *item {
	keyString := r.keyString(key)

	start := time.Now()
	r.lock.RLock()
	defer func() {
		r.lock.RUnlock()
		r.read += time.Since(start)
	}()

	return r.m[keyString]
}

func (r *insideCache) store(key interface{}, v interface{}, duration time.Duration) int32 {
	start := time.Now()

	r.lock.RLock()
	keyString := r.keyString(key)
	if r.m[keyString] != nil {
		r.lock.RUnlock()
		r.read += time.Since(start)
		return 0
	}

	r.lock.RUnlock()
	r.read += time.Since(start)

	stored := &item{
		v:    v,
		time: time.Now().Add(duration),
	}

	start = time.Now()
	r.lock.Lock()
	r.m[keyString] = stored
	r.lock.Unlock()
	r.write += time.Since(start)

	return 1
}

func (r *insideCache) refresh() int32 {
	keys := make([]string, 0, r.keyBufferSize)

	now := time.Now()

	r.lock.RLock()
	for k, v := range r.m {
		if now.After(v.time) {
			keys = append(keys, k)
		}
	}
	r.lock.RUnlock()
	r.read += time.Since(now)

	start := time.Now()
	r.lock.Lock()
	for _, key := range keys {
		delete(r.m, key)
	}
	r.lock.Unlock()

	r.write += time.Since(start)

	return int32(len(r.m))
}
