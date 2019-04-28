package gocache

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyCache(t *testing.T) {
	cache := NewExampleCache()

	req, _ := http.NewRequest(http.MethodGet, "test", nil)
	req.Header.Set("Authorization", "Bearer test_token")
	resp := &http.Response{}

	ret := cache.Get(req)
	assert.Nil(t, ret)

	cache.Store(req, resp)
	ret = cache.Get(req)
	assert.NotNil(t, ret)

	req.Header.Set("Authorization", "Bearer test_token2")
	ret = cache.Get(req)
	assert.Nil(t, ret)
}