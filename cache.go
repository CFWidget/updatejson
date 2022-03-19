package main

import (
	"os"
	"sync"
	"time"
)

type CachedResponse struct {
	Data     interface{}
	ExpireAt time.Time
	Status   int
}

var cacheTtl time.Duration
var memcache = sync.Map{}

func init() {
	envCache := os.Getenv("CACHE_TTL")
	cacheTtl = time.Hour
	if envCache != "" {
		var err error
		cacheTtl, err = time.ParseDuration(envCache)
		if err != nil {
			panic(err)
		}
	}
}

func GetFromCache(key string) (CachedResponse, bool) {
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

func SetInCache(key string, status int, data interface{}) {
	memcache.Store(key, CachedResponse{Data: data, Status: status, ExpireAt: time.Now().Add(cacheTtl)})
}

func RemoveFromCache(key string) {
	memcache.Delete(key)
}

func cleanCache() {
	memcache.Range(func(k, v interface{}) bool {
		res, ok := v.(CachedResponse)
		if !ok || time.Now().After(res.ExpireAt) {
			memcache.Delete(k)
			return true
		}

		return true
	})
}