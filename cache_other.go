//go:build !windows && !darwin

package main

// scanCacheEntries 在不支持的平台返回空。
func scanCacheEntries() []CacheEntry { return nil }
