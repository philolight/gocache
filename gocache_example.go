package gocache

import (
	"net/http"
	"time"
)

const(
	exampleCacheSize = 10000
	exampleBucketSize = 100
	exampleRefreshDuration = time.Second
)

type VerifyCache struct {
	cacher Cacher
}

func NewExampleCache() *VerifyCache {
	ret := &VerifyCache{}
	ret.cacher = New(ret.shortKey, ret.key, exampleRefreshDuration, exampleCacheSize, exampleBucketSize)
	ret.start()

	return ret
}

func (r *VerifyCache) start() {
	go r.cacher.Start()
}

func (r *VerifyCache) Get(req *http.Request) *http.Response {
	if r == nil { // null object pattern
		return nil
	}

	stored, ok := r.cacher.Get(req).(*http.Response)
	if !ok {
		return nil
	}

	return stored
}

func (r *VerifyCache) Store(req *http.Request, resp *http.Response) {
	if r == nil { // null object pattern
		return
	}

	r.cacher.Store(req, resp, r.cacheDuration(resp.StatusCode))
}

func (r *VerifyCache) shortKey(key interface{}) uint {
	req, ok := key.(*http.Request)
	if !ok {
		return 0
	}
	keyBytes := []byte(req.Header.Get("Authorization"))

	len := len(keyBytes)

	var sum uint
	for i := 0; i < len; i++ {
		sum += uint(keyBytes[i])
	}

	return sum
}

func (r *VerifyCache) key(key interface{}) string {
	req, ok := key.(*http.Request)
	if !ok {
		return ""
	}
	return req.Header.Get("Authorization")
}

func (r *VerifyCache) cacheDuration(statusCode int) time.Duration {
	if statusCode == http.StatusUnauthorized {
		return time.Minute
	}

	return time.Second * 5
}
