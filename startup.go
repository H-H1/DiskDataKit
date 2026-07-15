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
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// StartupItem 描述一个启动项或系统插件。
type StartupItem struct {
	Category string `json:"category"` // logon/explorer/ie
	Name     string `json:"name"`     // 原始名称
	ZhName   string `json:"zhName"`   // 中文名（AI 生成，可为空）
	Path     string `json:"path"`     // 命令/CLSID 路径
	Location string `json:"location"` // 注册表位置或文件位置
	State    string `json:"state"`    // enabled/disabled
	IsSystem bool   `json:"isSystem"` // 是否系统自带项（AI 判断）
}

// startupCacheRecord 缓存记录元信息（列表用）。
type startupCacheRecord struct {
	ID         string `json:"id"`
	SavedAt    string `json:"savedAt"`
	ItemsCount int    `json:"itemsCount"`
	SysCount   int    `json:"sysCount"`
	AppCount   int    `json:"appCount"`
}

// startupCacheData 单条缓存完整数据。
type startupCacheData struct {
	SavedAt string        `json:"savedAt"`
	Items   []StartupItem `json:"items"`
}

// startupCacheDir 返回启动项缓存目录路径。
func startupCacheDir() string {
	var dir string
	switch {
	case isWindows:
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "DiskDataKit", "startup_cache")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Caches", "DiskDataKit", "startup_cache")
	default:
		dir = os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = filepath.Join(os.Getenv("HOME"), ".cache")
		}
		dir = filepath.Join(dir, "DiskDataKit", "startup_cache")
	}
	return dir
}

// 类别中文映射
var startupCategoryNames = map[string]string{
	"logon":    "登录启动项",
	"explorer": "资源管理器插件",
	"ie":       "IE 加载项",
}

// handleStartupList 返回所有启动项/系统插件，并将结果缓存到本地文件。
func handleStartupList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	items := listStartupItems()
	// 并发调用大模型为每项生成中文名（失败则保留原名）
	translateStartupNames(items)

	// 保存到缓存目录（每次扫描生成独立文件）
	now := time.Now()
	id := now.Format("20060102_150405")
	cache := startupCacheData{
		SavedAt: now.Format("2006-01-02 15:04:05"),
		Items:   items,
	}
	cacheDir := startupCacheDir()
	_ = os.MkdirAll(cacheDir, 0755)
	if data, err := sonic.Marshal(cache); err == nil {
		_ = os.WriteFile(filepath.Join(cacheDir, id+".json"), data, 0644)
	}

	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// handleStartupCache 管理启动项缓存记录。
// GET /api/startup/cache          -> 列出所有缓存记录
// GET /api/startup/cache?id=xxx   -> 查看指定记录的详细内容
// DELETE /api/startup/cache?id=xxx -> 删除指定记录
// DELETE /api/startup/cache        -> 删除全部记录
func handleStartupCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cacheDir := startupCacheDir()
	id := r.URL.Query().Get("id")

	if r.Method == http.MethodDelete {
		if id != "" {
			// 删除指定记录
			path := filepath.Join(cacheDir, id+".json")
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}
		} else {
			// 删除全部
			_ = os.RemoveAll(cacheDir)
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}

	// GET
	if id != "" {
		// 查看指定记录详情
		data, err := os.ReadFile(filepath.Join(cacheDir, id+".json"))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": "记录不存在"})
			return
		}
		var cache startupCacheData
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
		json.NewEncoder(w).Encode(map[string]any{"records": []startupCacheRecord{}})
		return
	}

	var records []startupCacheRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		recordID := strings.TrimSuffix(e.Name(), ".json")
		// 读取文件获取元信息
		data, err := os.ReadFile(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var cache startupCacheData
		if err := sonic.Unmarshal(data, &cache); err != nil {
			continue
		}
		sysCount := 0
		for _, it := range cache.Items {
			if it.IsSystem {
				sysCount++
			}
		}
		records = append(records, startupCacheRecord{
			ID:         recordID,
			SavedAt:    cache.SavedAt,
			ItemsCount: len(cache.Items),
			SysCount:   sysCount,
			AppCount:   len(cache.Items) - sysCount,
		})
	}

	// 按时间倒序（新的在前）
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID > records[j].ID
	})

	json.NewEncoder(w).Encode(map[string]any{"records": records})
}

// handleStartupToggle 启用/禁用指定启动项。
func handleStartupToggle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Category string `json:"category"`
		Name     string `json:"name"`
		Location string `json:"location"`
		Enable   bool   `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if err := toggleStartupItem(req.Category, req.Name, req.Location, req.Enable); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// translateStartupNames 分批调用大模型为启动项生成中文名并判断是否系统项，每组 50 个合并请求。
// 未配置 AI 或调用失败时静默跳过（ZhName 留空，前端回退显示 Name）。
// 每组的请求上下文、响应内容和耗时记录到 log/startup.log。
func translateStartupNames(items []StartupItem) {
	if len(items) == 0 {
		return
	}
	client := getCurrentAIClient()
	if client == nil {
		return
	}

	// 初始化日志文件（仅开发模式）
	var logger *os.File
	if devMode {
		logFile := filepath.Join("log", "startup.log")
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

	writeLog("========== %s 开始翻译 %d 个启动项 ==========\n", time.Now().Format("2006-01-02 15:04:05"), len(items))

	const batchSize = 50
	sem := make(chan struct{}, 10) // 组并发上限
	var wg sync.WaitGroup

	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]
		offset := start
		batchNum := start/batchSize + 1

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 构建批量翻译 prompt
			var sb strings.Builder
			sb.WriteString("以下是 Windows 启动项列表，请为每一项生成简短中文名并判断是否为系统自带项。\n")
			sb.WriteString("规则：\n")
			sb.WriteString("- zhName：已有中文名保留原样；英文程序名翻译为中文；CLSID/路径根据信息推断所属软件中文名；无法判断的返回原名称。\n")
			sb.WriteString("- isSystem：Windows 系统自带的启动项/服务（如 ctfmon、SecurityHealth、OneDriveSetup 系统目录等）为 true，第三方软件为 false。\n\n")
			sb.WriteString("输入列表：\n")
			for i, it := range batch {
				catName := startupCategoryNames[it.Category]
				if catName == "" {
					catName = it.Category
				}
				sb.WriteString(fmt.Sprintf("%d. 类别:%s 名称:%s 路径:%s 位置:%s\n",
					i+1, catName, it.Name, it.Path, it.Location))
			}
			sb.WriteString("\n请严格只输出 JSON 数组，不要任何解释或 markdown 标记，格式如下：\n")
			sb.WriteString(`[{"index":1,"zhName":"中文名","isSystem":false}]`)

			prompt := sb.String()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			reqStart := time.Now()
			resp, err := client.Chat(ctx, "", prompt, aiCurrentProvider)
			duration := time.Since(reqStart)

			if err != nil {
				writeLog("[%s] 第 %d 组 (项 %d-%d) 请求失败 耗时: %s 错误: %v\n",
					time.Now().Format("15:04:05"), batchNum, offset+1, offset+len(batch), duration, err)
				writeLog("  Prompt:\n%s\n\n", prompt)
				return
			}
			if resp == nil || len(resp.Choices) == 0 {
				writeLog("[%s] 第 %d 组 (项 %d-%d) 响应为空 耗时: %s\n",
					time.Now().Format("15:04:05"), batchNum, offset+1, offset+len(batch), duration)
				writeLog("  Prompt:\n%s\n\n", prompt)
				return
			}

			content := strings.TrimSpace(resp.Choices[0].Message.Content)
			writeLog("[%s] 第 %d 组 (项 %d-%d) 耗时: %s\n",
				time.Now().Format("15:04:05"), batchNum, offset+1, offset+len(batch), duration)
			writeLog("  Prompt:\n%s\n", prompt)
			writeLog("  Response:\n%s\n\n", content)

			// 去除可能的 markdown 代码块标记
			content = strings.TrimPrefix(content, "```json")
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimSuffix(content, "```")
			content = strings.TrimSpace(content)

			// 解析 JSON 数组
			var results []struct {
				Index    int    `json:"index"`
				ZhName   string `json:"zhName"`
				IsSystem bool   `json:"isSystem"`
			}
			if err := sonic.UnmarshalString(content, &results); err != nil {
				writeLog("  JSON 解析失败: %v\n\n", err)
				return
			}
			for _, r := range results {
				idx := r.Index - 1 // 转为 0-based
				if idx < 0 || idx >= len(batch) {
					continue
				}
				zh := strings.Trim(r.ZhName, "\"'`「」")
				if zh != "" {
					items[offset+idx].ZhName = zh
				}
				items[offset+idx].IsSystem = r.IsSystem
			}
		}()
	}
	wg.Wait()
	writeLog("========== %s 翻译完成 ==========\n\n", time.Now().Format("2006-01-02 15:04:05"))
}
