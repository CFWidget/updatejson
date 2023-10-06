package main

import (
	"github.com/cfwidget/updatejson/env"
	"sync"
	"time"
)

type CachedResponse struct {
	Data     interface{}
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
		c := time.NewTicker(time.Hour)
		select {
		case <-c.C:
			cleanCache()
		}
	}()
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

func SetInCache(key string, status int, data interface{}) time.Time {
	cache := CachedResponse{Data: data, Status: status, ExpireAt: time.Now().Add(cacheTtl)}
	memcache.Store(key, cache)
	return cache.ExpireAt
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
