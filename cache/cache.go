package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/cfwidget/updatejson/env"
	"github.com/gin-gonic/gin"
)

type CachedResponse struct {
	Data     any
	ExpireAt time.Time
	Status   int
}

var cacheTtl time.Duration
var memcache sync.Map

func init() {
	envCache := env.Get("CACHE_TTL")
	cacheTtl = time.Hour
	if envCache != "" {
		var err error
		cacheTtl, err = time.ParseDuration(envCache)
		if err != nil {
			panic(err)
		}
	}

	go func() {
		c := time.NewTicker(5 * time.Minute)
		select {
		case <-c.C:
			cleanCache()
		}
	}()
}

func AddHeaders(c *gin.Context, cacheExpireTime time.Time) {
	maxAge := cacheTtl.Seconds()
	age := cacheTtl.Seconds() - cacheExpireTime.Sub(time.Now()).Seconds()

	c.Header("Cache-Control", fmt.Sprintf("max-age=%.0f, public", maxAge))
	c.Header("Age", fmt.Sprintf("%.0f", age))
	c.Header("MemCache-Expires-At", cacheExpireTime.UTC().Format(time.RFC3339))
}

func Get(key string) (CachedResponse, bool) {
	val, exists := memcache.Load(key)
	if !exists {
		return CachedResponse{}, false
	}

	res, ok := val.(CachedResponse)
	if !ok || time.Now().After(res.ExpireAt) {
		memcache.Delete(key)
		return CachedResponse{}, false
	}

	return res, true
}

func GetByRequest(c *gin.Context) (CachedResponse, bool) {
	return Get(GetKey(c))
}

func Set(key string, status int, data any) time.Time {
	cache := CachedResponse{Data: data, Status: status, ExpireAt: time.Now().Add(cacheTtl)}
	memcache.Store(key, cache)
	return cache.ExpireAt
}

func Remove(key string) {
	memcache.Delete(key)
}

func GetKey(c *gin.Context) string {
	return c.Request.Host + c.Request.RequestURI
}

func cleanCache() {
	memcache.Range(func(k, v any) bool {
		res, ok := v.(CachedResponse)
		if !ok || time.Now().After(res.ExpireAt) {
			memcache.Delete(k)
			return true
		}

		return true
	})
}
