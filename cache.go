package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// cacheDelResult 清理单个缓存目录的结果。
type cacheDelResult struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Failed int    `json:"failed"`
	Error  string `json:"error,omitempty"`
}

// CacheEntry 描述一个扫描到的缓存目录。
type CacheEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Count  int    `json:"count"`
	ZhName string `json:"zhName"` // AI 识别的软件中文名
	Safe   bool   `json:"safe"`   // AI 判断是否安全清理
	Desc   string `json:"desc"`   // AI 描述
}

// cacheCacheRecord 缓存扫描记录元信息（列表用）。
type cacheCacheRecord struct {
	ID         string `json:"id"`
	SavedAt    string `json:"savedAt"`
	ItemsCount int    `json:"itemsCount"`
	TotalSize  int64  `json:"totalSize"`
	SafeCount  int    `json:"safeCount"`
	WarnCount  int    `json:"warnCount"`
}

// cacheCacheData 单条缓存扫描完整数据。
type cacheCacheData struct {
	SavedAt string       `json:"savedAt"`
	Items   []CacheEntry `json:"items"`
}

// cacheCacheDir 返回缓存扫描记录的缓存目录路径。
func cacheCacheDir() string {
	var dir string
	switch {
	case isWindows:
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "DiskDataKit", "cache_records")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Caches", "DiskDataKit", "cache_records")
	default:
		dir = os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = filepath.Join(os.Getenv("HOME"), ".cache")
		}
		dir = filepath.Join(dir, "DiskDataKit", "cache_records")
	}
	return dir
}

// handleCacheList 扫描系统缓存目录，返回各项（含 AI 识别与大小），并将结果缓存到本地文件。
func handleCacheList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	maxDepth := 3
	if d := r.URL.Query().Get("depth"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			maxDepth = n
		}
	}
	entries := scanCacheEntries(maxDepth)
	scanEntrySizes(entries)
	// 过滤掉不存在或大小为 0 的项
	valid := make([]CacheEntry, 0, len(entries))
	for _, e := range entries {
		if e.Size > 0 {
			valid = append(valid, e)
		}
	}
	aiIdentifyCache(valid)

	// 可清理项置顶，再按大小降序
	sort.Slice(valid, func(i, j int) bool {
		if valid[i].Safe != valid[j].Safe {
			return valid[i].Safe
		}
		return valid[i].Size > valid[j].Size
	})

	// 保存到缓存目录（每次扫描生成独立文件）
	now := time.Now()
	id := now.Format("20060102_150405")
	cache := cacheCacheData{
		SavedAt: now.Format("2006-01-02 15:04:05"),
		Items:   valid,
	}
	cacheDir := cacheCacheDir()
	_ = os.MkdirAll(cacheDir, 0755)
	if data, err := sonic.Marshal(cache); err == nil {
		_ = os.WriteFile(filepath.Join(cacheDir, id+".json"), data, 0644)
	}

	json.NewEncoder(w).Encode(map[string]any{"items": valid})
}

// handleCacheCache 管理缓存扫描记录。
// GET /api/cache/cache          -> 列出所有缓存记录
// GET /api/cache/cache?id=xxx   -> 查看指定记录的详细内容
// DELETE /api/cache/cache?id=xxx -> 删除指定记录
// DELETE /api/cache/cache        -> 删除全部记录
func handleCacheCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cacheDir := cacheCacheDir()
	id := r.URL.Query().Get("id")

	if r.Method == http.MethodDelete {
		if id != "" {
			path := filepath.Join(cacheDir, id+".json")
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}
		} else {
			_ = os.RemoveAll(cacheDir)
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}

	// GET
	if id != "" {
		data, err := os.ReadFile(filepath.Join(cacheDir, id+".json"))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": "记录不存在"})
			return
		}
		var cache cacheCacheData
		if err := sonic.Unmarshal(data, &cache); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": "解析失败"})
			return
		}
		json.NewEncoder(w).Encode(cache)
		return
	}

	// 列出所有缓存记录
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"records": []cacheCacheRecord{}})
		return
	}

	var records []cacheCacheRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		recordID := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var cache cacheCacheData
		if err := sonic.Unmarshal(data, &cache); err != nil {
			continue
		}
		var totalSize int64
		safeCnt, warnCnt := 0, 0
		for _, it := range cache.Items {
			totalSize += it.Size
			if it.Safe {
				safeCnt++
			} else {
				warnCnt++
			}
		}
		records = append(records, cacheCacheRecord{
			ID:         recordID,
			SavedAt:    cache.SavedAt,
			ItemsCount: len(cache.Items),
			TotalSize:  totalSize,
			SafeCount:  safeCnt,
			WarnCount:  warnCnt,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].ID > records[j].ID
	})

	json.NewEncoder(w).Encode(map[string]any{"records": records})
}

// handleCacheDelete 删除选中的缓存路径。
// 逐个删除目录内的文件，被占用的文件跳过，不影响其他文件清理。
// 清理完成后更新对应缓存记录文件中各项的大小与数量。
func handleCacheDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Paths    []string `json:"paths"`
		RecordID string   `json:"recordId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	results := make([]cacheDelResult, 0, len(req.Paths))
	var totalCleaned int64
	cleanedPaths := make(map[string]bool, len(req.Paths))

	for _, p := range req.Paths {
		cleaned, failed := cleanDirFiles(p)

		cleanedPaths[p] = true
		r := cacheDelResult{Path: p, Size: cleaned, Failed: failed}
		if failed > 0 {
			r.Error = fmt.Sprintf("%d 个文件被占用或右键打开文件夹手动清理", failed)
		}
		results = append(results, r)
		totalCleaned += cleaned
	}

	// 更新缓存记录文件
	var updatedItems []CacheEntry
	if req.RecordID != "" {
		recordPath := filepath.Join(cacheCacheDir(), req.RecordID+".json")
		if data, err := os.ReadFile(recordPath); err == nil {
			var cache cacheCacheData
			if sonic.Unmarshal(data, &cache) == nil {
				for _, item := range cache.Items {
					if !cleanedPaths[item.Path] {
						// 未清理的项保持原样
						updatedItems = append(updatedItems, item)
						continue
					}
					// 已清理的项，重新计算剩余大小
					newSize, newCount := calcDirSize(item.Path)
					if newSize > 0 {
						item.Size = newSize
						item.Count = newCount
						updatedItems = append(updatedItems, item)
					}
					// newSize == 0 则从记录中移除
				}
				cache.Items = updatedItems
				if out, err := sonic.Marshal(cache); err == nil {
					_ = os.WriteFile(recordPath, out, 0644)
				}
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"results":      results,
		"totalCleaned": totalCleaned,
		"failedCount":  countFailedResults(results),
		"updatedItems": updatedItems,
	})
}

// countFailedResults 统计有失败文件的目录数。
func countFailedResults(results []cacheDelResult) int {
	count := 0
	for _, r := range results {
		if r.Failed > 0 {
			count++
		}
	}
	return count
}

// cleanDirFiles 逐个删除目录内的文件，返回已清理大小和失败文件数。
// 被占用的文件跳过，不影响其他文件。
func cleanDirFiles(dir string) (cleanedSize int64, failedCount int) {
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			// 先不删除目录本身，等子文件清空后再删
			return nil
		}
		info, err := d.Info()
		if err != nil {
			failedCount++
			return nil
		}
		if err := os.Remove(p); err != nil {
			failedCount++
		} else {
			cleanedSize += info.Size()
		}
		return nil
	})
	// 清理空目录（非递归，只删已变空的目录）
	cleanEmptyDirs(dir)
	return
}

// cleanEmptyDirs 自底向上删除变空的目录。
func cleanEmptyDirs(dir string) {
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == dir {
			return nil
		}
		// 先递归处理子目录（WalkDir 是前序遍历，这里收集后自底向上删）
		return nil
	})
	// 第二轮：自底向上删空目录
	var dirs []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		dirs = append(dirs, p)
		return nil
	})
	// 从最深到最浅
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(d)
		}
	}
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

// aiIdentifyCache 批量调用大模型识别缓存目录归属的软件及是否安全清理。
// 采用分批策略：每批 25 个路径一次性请求，减少 API 调用次数，避免限流。
// 同时启动本地规则识别作为兜底，确保每个 entry 都有中文名。
// prompt 中会载入注册表已安装软件列表作为参考，提高识别准确率。
// 每批的请求上下文、响应内容和耗时记录到 log/cache.log。
func aiIdentifyCache(entries []CacheEntry) {
	// 初始化日志文件（仅开发模式）
	var logger *os.File
	if devMode {
		logFile := filepath.Join("log", "cache.log")
		if err := os.MkdirAll("log", 0755); err != nil {
			log.Printf("创建日志目录失败: %v", err)
		}
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("打开日志文件失败: %v", err)
		} else {
			logger = f
			defer logger.Close()
		}
	}
	var logMu sync.Mutex
	writeLog := func(format string, args ...any) {
		if logger == nil {
			return
		}
		logMu.Lock()
		defer logMu.Unlock()
		fmt.Fprintf(logger, format, args...)
	}

	if len(entries) == 0 {
		writeLog("========== %s 无缓存目录需要识别 ==========\n\n", time.Now().Format("2006-01-02 15:04:05"))
		return
	}

	writeLog("========== %s 开始识别 %d 个缓存目录 ==========\n", time.Now().Format("2006-01-02 15:04:05"), len(entries))

	// 1. 本地规则预识别（兜底，确保每项都有中文名）
	for i := range entries {
		entries[i].ZhName, entries[i].Safe, entries[i].Desc = localIdentifyCache(entries[i].Path)
	}

	client := getCurrentAIClient()
	if client == nil {
		fmt.Println("[cache] AI 未配置，仅使用本地规则识别")
		writeLog("AI 未配置，仅使用本地规则识别\n")
		writeLog("========== %s 识别完成 (仅本地规则) ==========\n\n", time.Now().Format("2006-01-02 15:04:05"))
		return
	}

	// 2. 分批调用 AI 增强识别
	const batchSize = 25
	sem := make(chan struct{}, 500) // 最多 500 个并发批次
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for start := 0; start < len(entries); start += batchSize {
		end := start + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[start:end]
		batchNum := start/batchSize + 1

		wg.Add(1)
		go func(b []CacheEntry, baseIdx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 构建路径列表
			var pathList strings.Builder
			for i, e := range b {
				pathList.WriteString(fmt.Sprintf("%d. %s\n", i, e.Path))
			}

			prompt := fmt.Sprintf(`根据缓存目录路径判断每个目录属于哪个软件，以及是否安全清理。
路径列表：
%s
规则：
- 临时缓存（网页缓存、缩略图、编译缓存、日志、CrashDumps等）safe=true
- 可能含登录态/用户数据/配置/收藏夹的目录 safe=false
- zhName 填写软件的原始名称（如"WeChat"、"Google Chrome"、"Visual Studio Code"），不要翻译成中文，无法识别的用"Unknown"
只输出 JSON 数组，不要 markdown 代码块，不要解释：
[{"zhName":"软件名称","safe":true,"desc":"简短说明"},...]`, pathList.String())

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			reqStart := time.Now()
			resp, err := client.Chat(ctx, "", prompt, aiCurrentProvider)
			duration := time.Since(reqStart)

			if err != nil || resp == nil || len(resp.Choices) == 0 {
				writeLog("[%s] 第 %d 批 (项 %d-%d) 请求失败 耗时: %s 错误: %v\n  Prompt:\n%s\n\n",
					time.Now().Format("15:04:05"), batchNum, baseIdx+1, baseIdx+len(b), duration, err, prompt)
				fmt.Printf("[cache] AI 批次识别失败 (base=%d): %v\n", baseIdx, err)
				return
			}

			content := stripJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
			writeLog("[%s] 第 %d 批 (项 %d-%d) 耗时: %s\n  Prompt:\n%s\n  Response:\n%s\n\n",
				time.Now().Format("15:04:05"), batchNum, baseIdx+1, baseIdx+len(b), duration, prompt, content)

			var results []struct {
				ZhName string `json:"zhName"`
				Safe   bool   `json:"safe"`
				Desc   string `json:"desc"`
			}
			if err := json.Unmarshal([]byte(content), &results); err != nil {
				writeLog("  JSON 解析失败: %v\n内容: %s\n\n", err, content)
				fmt.Printf("[cache] AI 批次 JSON 解析失败 (base=%d): %v\n内容: %s\n", baseIdx, err, content)
				return
			}

			mu.Lock()
			for i, r := range results {
				if i >= len(b) {
					break
				}
				idx := baseIdx + i
				if r.ZhName != "" {
					entries[idx].ZhName = r.ZhName
				}
				entries[idx].Safe = r.Safe
				if r.Desc != "" {
					entries[idx].Desc = r.Desc
				}
				successCount++
			}
			mu.Unlock()
		}(batch, start)
	}
	wg.Wait()

	writeLog("========== %s 识别完成 (成功 %d/%d) ==========\n\n", time.Now().Format("2006-01-02 15:04:05"), successCount, len(entries))
	fmt.Printf("[cache] AI 识别完成: %d/%d 项成功\n", successCount, len(entries))
}

// localIdentifyCache 基于路径关键词的本地规则识别（兜底方案）。
func localIdentifyCache(path string) (zhName string, safe bool, desc string) {
	p := strings.ToLower(path)
	safe = true // 默认可清理

	switch {
	case strings.Contains(p, "chrome"):
		return "Chrome浏览器", true, "浏览器缓存"
	case strings.Contains(p, "edge"):
		return "Edge浏览器", true, "浏览器缓存"
	case strings.Contains(p, "firefox"):
		return "Firefox浏览器", true, "浏览器缓存"
	case strings.Contains(p, "\\wechat") || strings.Contains(p, "/wechat"):
		return "微信", false, "可能含聊天记录"
	case strings.Contains(p, "tencent\\qq") || strings.Contains(p, "tencent/qq"):
		return "QQ", false, "可能含聊天记录"
	case strings.Contains(p, "dingtalk"):
		return "钉钉", false, "可能含登录态"
	case strings.Contains(p, "wps"):
		return "WPS Office", true, "Office 缓存"
	case strings.Contains(p, "code\\cache") || strings.Contains(p, "code/cache"):
		return "Visual Studio Code", true, "编辑器缓存"
	case strings.Contains(p, "jetbrains") || strings.Contains(p, "intellij"):
		return "JetBrains IDE", true, "IDE 缓存"
	case strings.Contains(p, "nuget"):
		return "NuGet", true, "包管理器缓存"
	case strings.Contains(p, "npm") || strings.Contains(p, "node_modules"):
		return "Node.js", true, "包管理器缓存"
	case strings.Contains(p, "pip") || strings.Contains(p, "\\python"):
		return "Python", true, "包管理器缓存"
	case strings.Contains(p, "windows\\temp") || strings.Contains(p, "windows/temp"):
		return "Windows 临时文件", true, "系统临时目录"
	case strings.Contains(p, "\\temp"):
		return "临时文件", true, "临时目录"
	case strings.Contains(p, "crashdumps"):
		return "崩溃转储", true, "崩溃日志"
	case strings.Contains(p, "thumbnail") || strings.Contains(p, "缩略图"):
		return "缩略图缓存", true, "缩略图"
	case strings.Contains(p, "iconcache"):
		return "图标缓存", true, "图标缓存"
	case strings.Contains(p, "fontcache"):
		return "字体缓存", true, "字体缓存"
	default:
		// 从路径提取软件名
		parts := strings.Split(path, string(filepath.Separator))
		for i := len(parts) - 1; i >= 0; i-- {
			name := parts[i]
			if name == "" || name == "Cache" || name == "cache" || name == "Local" || name == "LocalAppData" {
				continue
			}
			return name + " 缓存", true, "缓存目录"
		}
		return "未知软件缓存", true, "缓存目录"
	}
}

// loadInstalledAppNames 从注册表读取已安装软件名称（去重后排序）。
// 作为 AI 识别缓存归属的参考上下文。
func loadInstalledAppNames() []string {
	items := scanInstalled()
	seen := make(map[string]bool)
	var names []string
	for _, it := range items {
		if it.Name == "" || seen[it.Name] {
			continue
		}
		seen[it.Name] = true
		names = append(names, it.Name)
	}
	sort.Strings(names)
	return names
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
