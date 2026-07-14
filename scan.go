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

// handleScanList 扫描系统并返回所有项（含 AI 判断结果）。
func handleScanList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	items := scanAll()
	aiJudgeScanItems(items)
	json.NewEncoder(w).Encode(map[string]any{"items": items})
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
			prompt := fmt.Sprintf(`当前是流氓软件检测的「%s」类别。判断以下项目是否可疑。
判断标准：
- 知名正规软件（微软、谷歌、Adobe 等）-> safe
- 路径在 AppData/Temp、无签名、名称含广告/加速/天气/壁纸/导航/卫士等 -> suspicious
- 明确流氓或恶意软件 -> suspicious
- 信息不足无法判断 -> unknown

项目信息：
名称：%s
路径：%s
发布者：%s
位置：%s

只输出 JSON，不要 markdown 代码块，不要解释：
{"zhName":"中文名","verdict":"safe|suspicious|unknown","reason":"简短理由"}`, catName, it.Name, it.Path, it.Publisher, it.Location)

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
