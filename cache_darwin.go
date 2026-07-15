//go:build darwin

package main

import (
	"os"
	"path/filepath"
)

// scanCacheEntries 扫描 macOS 缓存目录：~/Library/Caches 下的每个子目录、
// ~/.cache、/Library/Caches，以及 npm/pip 等开发工具缓存。
func scanCacheEntries(maxDepth int) []CacheEntry {
	var entries []CacheEntry
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// ~/Library/Caches 下每个子目录都是一个软件缓存
	entries = append(entries, scanChildren(filepath.Join(home, "Library", "Caches"))...)
	// /Library/Caches
	entries = append(entries, scanChildren("/Library/Caches")...)
	// ~/.cache
	entries = append(entries, scanChildren(filepath.Join(home, ".cache"))...)

	// 开发工具缓存
	devCaches := []string{
		filepath.Join(home, ".npm", "_cacache"),
		filepath.Join(home, "Library", "Caches", "pip"),
		filepath.Join(home, "Library", "Caches", "go-build"),
		filepath.Join(home, ".gradle", "caches"),
		filepath.Join(home, ".m2", "repository"),
	}
	for _, p := range devCaches {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			entries = append(entries, CacheEntry{Path: p})
		}
	}

	// 去重
	seen := make(map[string]bool)
	out := make([]CacheEntry, 0, len(entries))
	for _, e := range entries {
		if !seen[e.Path] {
			seen[e.Path] = true
			out = append(out, e)
		}
	}
	return out
}

// scanChildren 列出目录的直接子目录，每个作为一个 CacheEntry。
func scanChildren(dir string) []CacheEntry {
	var out []CacheEntry
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, CacheEntry{Path: filepath.Join(dir, e.Name())})
		}
	}
	return out
}
