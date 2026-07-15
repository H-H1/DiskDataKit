package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// ScanItem 描述一个被扫描的程序/文件/启动项，用于流氓软件识别。
type ScanItem struct {
	Category  string `json:"category"`  // installed/process/file/startup
	Name      string `json:"name"`      // 原始名称
	ZhName    string `json:"zhName"`    // 中文名（AI 生成）
	Path      string `json:"path"`      // 可执行文件路径
	Publisher string `json:"publisher"` // 发布者/签名
	Location  string `json:"location"`  // 来源（注册表位置/目录等）
	Verdict   string `json:"verdict"`   // safe/suspicious/unknown
	Reason    string `json:"reason"`    // AI 判断理由
	CPUUsage  string `json:"cpuUsage"`  // CPU 占用百分比（仅 process 类别）
	MemUsage  string `json:"memUsage"`  // 内存占用 MB（仅 process 类别）
}

// 类别中文映射
var scanCategoryNames = map[string]string{
	"installed": "已安装程序",
	"process":   "运行中进程",
	"file":      "可疑文件",
	"startup":   "启动项",
}

// scanCacheRecord 缓存记录元信息（列表用）。
type scanCacheRecord struct {
	ID           string `json:"id"`
	SavedAt      string `json:"savedAt"`
	ItemsCount   int    `json:"itemsCount"`
	SafeCount    int    `json:"safeCount"`
	SusCount     int    `json:"susCount"`
	UnknownCount int    `json:"unknownCount"`
}

// scanCacheData 单条缓存完整数据。
type scanCacheData struct {
	SavedAt string     `json:"savedAt"`
	Items   []ScanItem `json:"items"`
}

// scanCacheDir 返回流氓软件扫描缓存目录路径。
func scanCacheDir() string {
	var dir string
	switch {
	case isWindows:
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "DiskDataKit", "scan_cache")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Caches", "DiskDataKit", "scan_cache")
	default:
		dir = os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = filepath.Join(os.Getenv("HOME"), ".cache")
		}
		dir = filepath.Join(dir, "DiskDataKit", "scan_cache")
	}
	return dir
}

// handleScanList 扫描系统并返回所有项（含 AI 判断结果），同时缓存到本地文件。
func handleScanList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	items := scanAll()
	aiJudgeScanItems(items)

	// 可疑项置顶：suspicious -> unknown -> safe
	sort.SliceStable(items, func(i, j int) bool {
		return scanVerdictRank(items[i].Verdict) < scanVerdictRank(items[j].Verdict)
	})

	// 保存到缓存目录
	now := time.Now()
	id := now.Format("20060102_150405")
	cache := scanCacheData{
		SavedAt: now.Format("2006-01-02 15:04:05"),
		Items:   items,
	}
	cacheDir := scanCacheDir()
	_ = os.MkdirAll(cacheDir, 0755)
	if data, err := sonic.Marshal(cache); err == nil {
		_ = os.WriteFile(filepath.Join(cacheDir, id+".json"), data, 0644)
	}

	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

// handleScanCache 管理流氓软件扫描缓存记录。
// GET /api/scan/cache          -> 列出所有缓存记录
// GET /api/scan/cache?id=xxx   -> 查看指定记录的详细内容
// DELETE /api/scan/cache?id=xxx -> 删除指定记录
// DELETE /api/scan/cache        -> 删除全部记录
func handleScanCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	cacheDir := scanCacheDir()
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
		var cache scanCacheData
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
		json.NewEncoder(w).Encode(map[string]any{"records": []scanCacheRecord{}})
		return
	}

	var records []scanCacheRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		recordID := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			continue
		}
		var cache scanCacheData
		if err := sonic.Unmarshal(data, &cache); err != nil {
			continue
		}
		safeCnt, susCnt, unknownCnt := 0, 0, 0
		for _, it := range cache.Items {
			switch it.Verdict {
			case "safe":
				safeCnt++
			case "suspicious":
				susCnt++
			default:
				unknownCnt++
			}
		}
		records = append(records, scanCacheRecord{
			ID:           recordID,
			SavedAt:      cache.SavedAt,
			ItemsCount:   len(cache.Items),
			SafeCount:    safeCnt,
			SusCount:     susCnt,
			UnknownCount: unknownCnt,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].ID > records[j].ID
	})

	json.NewEncoder(w).Encode(map[string]any{"records": records})
}

// scanVerdictRank 返回判定结果的排序权重，值越小越靠前。
func scanVerdictRank(verdict string) int {
	switch verdict {
	case "suspicious":
		return 0
	case "unknown":
		return 1
	default:
		return 2
	}
}

// aiJudgeScanItems 并发调用大模型判断每项是否可疑，填充 ZhName/Verdict/Reason。
func aiJudgeScanItems(items []ScanItem) {
	if len(items) == 0 {
		return
	}
	client := getCurrentAIClient()
	if client == nil {
		return
	}

	sem := make(chan struct{}, 500)
	var wg sync.WaitGroup

	for i := range items {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			it := items[idx]
			catName := scanCategoryNames[it.Category]
			if catName == "" {
				catName = it.Category
			}

			// 构建 prompt，进程类别附加 CPU/内存信息用于挖矿检测
			extraInfo := ""
			if it.Category == "process" && (it.CPUUsage != "" || it.MemUsage != "") {
				extraInfo = fmt.Sprintf("\nCPU 占用：%s\n内存占用：%s", it.CPUUsage, it.MemUsage)
			}

			prompt := fmt.Sprintf(`当前是流氓软件检测的「%s」类别。判断以下项目是否可疑。
判断标准：
- 知名正规软件（微软、谷歌、Adobe 等）-> safe
- 路径在 AppData/Temp、无签名、名称含广告/加速/天气/壁纸/导航/卫士等 -> suspicious
- 明确流氓或恶意软件 -> suspicious
- 信息不足无法判断 -> unknown
- 挖矿检测：进程 CPU 占用极高（>50%%）且名称不知名/路径可疑/非已知计算软件 -> suspicious
- 已知正规高 CPU 进程（如编译器、视频编码、3D 渲染、数据库）-> safe

项目信息：
名称：%s
路径：%s
发布者：%s
位置：%s%s

只输出 JSON，不要 markdown 代码块，不要解释：
{"zhName":"中文名","verdict":"safe|suspicious|unknown","reason":"简短理由"}`, catName, it.Name, it.Path, it.Publisher, it.Location, extraInfo)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := client.Chat(ctx, "", prompt, aiCurrentProvider)
			if err != nil || resp == nil || len(resp.Choices) == 0 {
				return
			}
			content := stripJSONFence(strings.TrimSpace(resp.Choices[0].Message.Content))
			var result struct {
				ZhName  string `json:"zhName"`
				Verdict string `json:"verdict"`
				Reason  string `json:"reason"`
			}
			if err := json.Unmarshal([]byte(content), &result); err != nil {
				return
			}
			if result.ZhName != "" {
				items[idx].ZhName = result.ZhName
			}
			if result.Verdict != "" {
				items[idx].Verdict = result.Verdict
			}
			if result.Reason != "" {
				items[idx].Reason = result.Reason
			}
		}(i)
	}
	wg.Wait()
}

// stripJSONFence 去除 AI 返回中可能的 markdown 代码块围栏（```json ... ```）。
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}
	return s
}
