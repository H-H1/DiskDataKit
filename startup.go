package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// StartupItem 描述一个启动项或系统插件。
type StartupItem struct {
	Category string `json:"category"` // logon/explorer/ie
	Name     string `json:"name"`     // 原始名称
	ZhName   string `json:"zhName"`   // 中文名（AI 生成，可为空）
	Path     string `json:"path"`     // 命令/CLSID 路径
	Location string `json:"location"` // 注册表位置或文件位置
	State    string `json:"state"`    // enabled/disabled
}

// 类别中文映射
var startupCategoryNames = map[string]string{
	"logon":    "登录启动项",
	"explorer": "资源管理器插件",
	"ie":       "IE 加载项",
}

// handleStartupList 返回所有启动项/系统插件。
func handleStartupList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	items := listStartupItems()
	// 并发调用大模型为每项生成中文名（失败则保留原名）
	translateStartupNames(items)
	json.NewEncoder(w).Encode(map[string]any{"items": items})
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

// translateStartupNames 并发调用大模型为每个启动项生成中文名，直接填充 item.ZhName。
// 未配置 AI 或单项调用失败时静默跳过（ZhName 留空，前端回退显示 Name）。
func translateStartupNames(items []StartupItem) {
	if len(items) == 0 {
		return
	}
	client := getCurrentAIClient()
	if client == nil {
		return
	}

	sem := make(chan struct{}, 8) // 并发上限，避免压垮 API
	var wg sync.WaitGroup

	for i := range items {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			it := items[idx]
			catName := startupCategoryNames[it.Category]
			if catName == "" {
				catName = it.Category
			}
			prompt := fmt.Sprintf(`当前是 Windows 启动项的「%s」类别。为该项生成一个简短的中文名（用于界面展示）：
- 名称已是中文或常见软件中文名，保留原样。
- 英文程序名翻译为中文（如 WeChat->微信, 360Tray->360安全卫士托盘, ctfmon->输入法指示器）。
- CLSID 或路径，根据路径/位置推断所属软件中文名；无法判断的返回原名称。
- 只输出中文名本身，不要解释、引号或标点。

名称：%s
路径：%s
位置：%s`, catName, it.Name, it.Path, it.Location)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := client.Chat(ctx, "", prompt, aiCurrentProvider)
			if err != nil || resp == nil || len(resp.Choices) == 0 {
				return
			}
			zh := strings.TrimSpace(resp.Choices[0].Message.Content)
			zh = strings.Trim(zh, "\"'`「」")
			if zh != "" {
				items[idx].ZhName = zh
			}
		}(i)
	}
	wg.Wait()
}
