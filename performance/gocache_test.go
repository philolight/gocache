package performance

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"testing"
	"encoding/base64"

	"oss.navercorp.com/JTF-P6/cic-server.git/.gopath/pkg/dep/sources/https---github.com-google-uuid"
)

const (
	limit               = 1000
	trials              = 100
	testRefreshDuration = time.Millisecond * 1000
	testCacheDuration   = time.Millisecond * 5000
	testBucketSize      = 251
)

var overLimit = make([]*http.Request, limit*10, limit*10)

func init() {
	for i := 0; i < len(overLimit); i++ {
		overLimit[i], _ = http.NewRequest(http.MethodGet, "test", nil)
		overLimit[i].Header.Add("Authorization", "Bearer "+RandomUUID())
	}
}

func TestBuckets(t *testing.T) {
	fmt.Println("SingleMapCache")
	ExampleSingleMapCache()
	fmt.Println("BucketSingleMapCache")
	ExampleBucketSingleMapCache()
	fmt.Println("BucketDoubleMapCache")
	ExampleBucketDoubleMapCache()
}

func ExampleBucketDoubleMapCache() {
	resp := &http.Response{}

	bc := &bucketCache{
		caches:          make([]BaseCache, testBucketSize, testBucketSize),
		size:            limit,
		bucketSize:      testBucketSize,
		capacity:        limit,
		shortKeyString:  shortKeyString,
		refreshDuration: testRefreshDuration,
		stop:            make(chan struct{}, 1),
	}

	for i := 0; i < testBucketSize; i++ {
		front := make(map[string]*item, limit/testBucketSize)
		back := make(map[string]*item, limit/testBucketSize)

		bc.caches[i] = BaseCache{
			keyString:     keyString,
			front:         &front,
			back:          &back,
			keyBufferSize: limit / testBucketSize,
		}
	}

	var res result

	go bc.Start()

	now := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < len(overLimit); i++ {
		wg.Add(1)
		req := overLimit[i]
		go request(bc, req, resp, trials, &wg, &res)
	}

	wg.Wait()
	bc.Stop()

	fmt.Println("time = ", time.Since(now), " hit = ", res.hit)

	fmt.Println("frontRead = ", bc.frontRead())
	fmt.Println("frontWrite = ", bc.frontWrite())
	fmt.Println("backRead = ", bc.backRead())
	fmt.Println("backWrite = ", bc.backWrite())
	fmt.Println("refresh = ", bc.refreshCount)
}

func ExampleBucketSingleMapCache() {
	resp := &http.Response{}

	bc := &singleBucketCache{
		caches:          make([]insideCache, testBucketSize, testBucketSize),
		size:            limit,
		bucketSize:      testBucketSize,
		capacity:        limit,
		shortKeyString:  shortKeyString,
		refreshDuration: testRefreshDuration,
		stop:            make(chan struct{}, 1),
	}

	for i := 0; i < testBucketSize; i++ {
		bc.caches[i] = insideCache{
			keyString:     keyString,
			m:             make(map[string]*item, limit/testBucketSize),
			keyBufferSize: limit / testBucketSize,
		}
	}

	var res result

	go bc.Start()

	now := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < len(overLimit); i++ {
		wg.Add(1)
		req := overLimit[i]
		go request(bc, req, resp, trials, &wg, &res)
	}

	wg.Wait()
	bc.Stop()

	fmt.Println("time = ", time.Since(now), " hit = ", res.hit)

	fmt.Println("read = ", bc.read())
	fmt.Println("write = ", bc.write())
	fmt.Println("refresh = ", bc.refreshCount)
}

func ExampleSingleMapCache() {
	resp := &http.Response{}
	mock := &singleMapCache{
		m:         make(map[string]*item, limit),
		keyString: keyString,
		stop:      make(chan struct{}, 1),

		size:            limit,
		refreshDuration: testRefreshDuration,
	}

	var res result

	go mock.Start()

	now := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < len(overLimit); i++ {
		wg.Add(1)
		req := overLimit[i]
		go request(mock, req, resp, trials, &wg, &res)
	}

	wg.Wait()
	mock.Stop()

	fmt.Println("time = ", time.Since(now), " hit = ", res.hit)

	fmt.Println("read = ", mock.read)
	fmt.Println("write = ", mock.write)
	fmt.Println("refresh = ", mock.refreshCount)
}

func keyString(key interface{}) string {
	req, ok := key.(*http.Request)
	if !ok {
		return ""
	}
	return req.Header.Get("Authorization")
}

func shortKeyString(key interface{}) uint {
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

type result struct {
	hit  int
	lock sync.RWMutex
}

func request(cache Cacher, req *http.Request, resp *http.Response, times int, wg *sync.WaitGroup, res *result) {
	defer wg.Done()

	var hit int

	for i := 0; i < times; i++ {
		ret := cache.Get(req)
		if ret == nil {
			time.Sleep(20 * time.Millisecond) // make response
			cache.Store(req, resp, testCacheDuration)
		} else {
			hit++
		}
		time.Sleep(10 * time.Millisecond)
	}

	res.lock.Lock()
	defer res.lock.Unlock()
	res.hit += hit
}

func RandomUUID() string {
	token, _ := uuid.NewRandom()
	return base64.RawURLEncoding.EncodeToString([]byte(token.String()))
}
