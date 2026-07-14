package util

import (
	"context"
	"io"
	"slices"

	"github.com/gin-gonic/gin"
)

const GinContextKey = "log"

func AreEqual(arr1, arr2 []string) bool {
	if arr1 == nil && arr2 == nil {
		return true
	}

	if arr1 == nil && arr2 != nil {
		return false
	}

	if arr2 == nil && arr1 != nil {
		return false
	}

	for _, a := range arr1 {
		if !slices.Contains(arr2, a) {
			return false
		}
	}

	for _, a := range arr2 {
		if !slices.Contains(arr1, a) {
			return false
		}
	}

	return true
}

func Dedup[T comparable](source []T) []T {
	result := make([]T, 0)

	for _, v := range source {
		if !slices.Contains(result, v) {
			result = append(result, v)
		}
	}

	return result
}

func Close(c io.Closer) {
	if c == nil {
		return
	}
	_ = c.Close()
}

func Context(c *gin.Context) context.Context {
	ctx, exists := c.Get(GinContextKey)
	if !exists {
		return c.Request.Context()
	}
	return ctx.(context.Context)
}
