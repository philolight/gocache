package performance

import (
	"sync"
	"time"
	"net/http"
	"fmt"
)

type singleMapCache struct {
	m         map[string]*item
	lock      sync.RWMutex
	keyString func(key interface{}) string
	stop      chan struct{}

	size            int
	refreshDuration time.Duration

	refreshCount int
	read         time.Duration
	write        time.Duration
}

func (r *singleMapCache) Get(key interface{}) interface{} {
	req := key.(*http.Request)
	keyString := r.keyString(req)

	start := time.Now()
	r.lock.RLock()
	item := r.m[keyString]
	r.lock.RUnlock()
	r.read += time.Since(start)

	if item == nil {
		return nil
	}

	return item.v
}

func (r *singleMapCache) Store(key interface{}, v interface{}, duration time.Duration) bool {
	start := time.Now()

	r.lock.RLock()

	if len(r.m) >= r.size {
		r.lock.RUnlock()
		r.read += time.Since(start)
		return false
	}

	keyString := r.keyString(key)
	if r.m[keyString] != nil {
		r.lock.RUnlock()
		r.read += time.Since(start)
		return false
	}

	r.lock.RUnlock()
	r.read += time.Since(start)

	item := &item{
		v:    v,
		time: time.Now().Add(duration),
	}

	start = time.Now()
	r.lock.Lock()
	r.m[keyString] = item
	r.lock.Unlock()
	r.write += time.Since(start)

	return true
}

func (r *singleMapCache) Start() {
	now := time.Now()
	for {
		select {
		case <-r.stop:
			return
		case <-time.After(r.refreshDuration):
			r.refresh()
			r.refreshCount++
			oldNow := now
			now = time.Now()
			fmt.Println("singleMapCache = ", now.Sub(oldNow))
		}
	}
}

func (r *singleMapCache) Stop() {
	r.stop <- struct{}{}
}

func (r *singleMapCache) refresh() {
	keys := make([]string, 0, (r.size/2)+1)

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
}