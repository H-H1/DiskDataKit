package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry 描述一个扫描到的缓存目录。
type CacheEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Count  int    `json:"count"`
	ZhName string `json:"zhName"` // AI 识别的软件中文名
	Safe   bool   `json:"safe"`   // AI 判断是否安全清理
	Desc   string `json:"desc"`   // AI 描述
}

// handleCacheList 扫描系统缓存目录，返回各项（含 AI 识别与大小）。
func handleCacheList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	entries := scanCacheEntries()
	scanEntrySizes(entries)
	// 过滤掉不存在或大小为 0 的项
	valid := make([]CacheEntry, 0, len(entries))
	for _, e := range entries {
		if e.Size > 0 {
			valid = append(valid, e)
		}
	}
	aiIdentifyCache(valid)
	json.NewEncoder(w).Encode(map[string]any{"items": valid})
}

// handleCacheDelete 删除选中的缓存路径。
func handleCacheDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	type delResult struct {
		Path  string `json:"path"`
		Size  int64  `json:"size"`
		Error string `json:"error,omitempty"`
	}
	results := make([]delResult, 0, len(req.Paths))
	for _, p := range req.Paths {
		sz := dirSize(p)
		err := os.RemoveAll(p)
		r := delResult{Path: p, Size: sz}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	json.NewEncoder(w).Encode(map[string]any{"results": results})
}

// scanEntrySizes 并发扫描各缓存目录大小。
func scanEntrySizes(entries []CacheEntry) {
	sem := make(chan struct{}, 500)
	var wg sync.WaitGroup
	for i := range entries {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			size, count := calcDirSize(entries[idx].Path)
			entries[idx].Size = size
			entries[idx].Count = count
		}(i)
	}
	wg.Wait()
}

// aiIdentifyCache 并发调用大模型识别每个缓存目录归属的软件及是否安全清理。
func aiIdentifyCache(entries []CacheEntry) {
	if len(entries) == 0 {
		return
	}
	client := getCurrentAIClient()
	if client == nil {
		return
	}
	sem := make(chan struct{}, 500)
	var wg sync.WaitGroup
	for i := range entries {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			prompt := fmt.Sprintf(`根据缓存目录路径判断它属于哪个软件，以及是否安全清理。
目录路径：%s
规则：
- 临时缓存（网页缓存、缩略图、编译缓存、日志等）safe=true
- 可能含登录态/用户数据/配置的目录 safe=false
只输出 JSON，不要 markdown 代码块，不要解释：
{"zhName":"软件中文名","safe":true,"desc":"简短说明"}`, entries[idx].Path)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := client.Chat(ctx, "", prompt, aiCurrentProvider)
			if err != nil || resp == nil || len(resp.Choices) == 0 {
				return
			}
			content := stripJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
			var res struct {
				ZhName string `json:"zhName"`
				Safe   bool   `json:"safe"`
				Desc   string `json:"desc"`
			}
			if err := json.Unmarshal([]byte(content), &res); err != nil {
				return
			}
			if res.ZhName != "" {
				entries[idx].ZhName = res.ZhName
			}
			entries[idx].Safe = res.Safe
			if res.Desc != "" {
				entries[idx].Desc = res.Desc
			}
		}(i)
	}
	wg.Wait()
}

// calcDirSize 递归计算目录大小和文件数。
func calcDirSize(dir string) (int64, int) {
	var total int64
	var count int
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
			count++
		}
		return nil
	})
	return total, count
}
