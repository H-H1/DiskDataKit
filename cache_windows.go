//go:build windows

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// scanCacheEntries 扫描 Windows 常见缓存目录：在 LocalAppData/AppData/ProgramData 下
// 查找名字含 "cache" 的目录（深度限制 5），以及 Temp、UWP LocalCache、浏览器缓存等。
func scanCacheEntries(maxDepth int) []CacheEntry {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	var entries []CacheEntry
	seen := make(map[string]bool)

	// 1. 在常见数据根目录下查找名字含 cache 的目录
	roots := []string{
		os.Getenv("LocalAppData"),
		os.Getenv("AppData"),
		os.Getenv("ProgramData"),
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			rel := strings.TrimPrefix(p, root)
			rel = strings.Trim(rel, string(filepath.Separator))
			depth := strings.Count(rel, string(filepath.Separator))
			if depth >= maxDepth {
				return filepath.SkipDir
			}
			name := strings.ToLower(d.Name())
			if isCacheDirName(name) {
				if !seen[p] {
					entries = append(entries, CacheEntry{Path: p})
					seen[p] = true
				}
				return filepath.SkipDir // 不深入缓存目录内部
			}
			return nil
		})
	}

	// 2. Temp 目录
	for _, t := range []string{os.Getenv("Temp"), os.Getenv("LocalAppData") + `\Temp`} {
		if t != "" && !seen[t] {
			if info, err := os.Stat(t); err == nil && info.IsDir() {
				entries = append(entries, CacheEntry{Path: t})
				seen[t] = true
			}
		}
	}

	// 3. UWP/商店应用 LocalCache
	packagesDir := os.Getenv("LocalAppData") + `\Packages`
	if entries2 := scanSubDirs(filepath.Join(packagesDir, "*", "LocalCache")); len(entries2) > 0 {
		entries = append(entries, entries2...)
	}

	return entries
}

// scanSubDirs 用 glob 模式匹配目录并加入 entries。
func scanSubDirs(pattern string) []CacheEntry {
	var out []CacheEntry
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && info.IsDir() {
			out = append(out, CacheEntry{Path: m})
		}
	}
	return out
}

// isCacheDirName 判断目录名是否像缓存目录。
func isCacheDirName(name string) bool {
	if name == "" {
		return false
	}
	known := map[string]bool{
		"cache": true, "caches": true, "cachedata": true, "cache2": true,
		"gpucache": true, "code cache": true, "service worker": true,
		"shadercache": true, "diskcache": true, "netcache": true,
	}
	if known[name] {
		return true
	}
	return strings.Contains(name, "cache")
}
