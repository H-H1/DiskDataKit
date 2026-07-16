package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CleanupTarget 清理目标
type CleanupTarget struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Desc     string `json:"desc"`
	Size     int64  `json:"size"`
	Count    int    `json:"count"`
	Scanning bool   `json:"scanning"`
}

// getCleanupTargets 返回当前平台的清理目标列表
func getCleanupTargets() []CleanupTarget {
	var targets []CleanupTarget

	switch {
	case isWindows:
		// 用户临时文件 %TEMP%
		targets = append(targets, CleanupTarget{
			ID:   "user_temp",
			Name: "用户临时文件",
			Path: os.TempDir(),
			Desc: "当前用户产生的临时文件，可放心清理",
		})

		// 系统临时文件 C:\Windows\Temp
		targets = append(targets, CleanupTarget{
			ID:   "sys_temp",
			Name: "系统临时文件",
			Path: `C:\Windows\Temp`,
			Desc: "系统运行产生的临时文件，可以安全删除",
		})

		// Windows 更新缓存
		targets = append(targets, CleanupTarget{
			ID:   "win_update",
			Name: "Windows 更新缓存",
			Path: `C:\Windows\SoftwareDistribution\Download`,
			Desc: "存放 Windows 更新下载的安装包，安装完成后可清理",
		})

		// 缩略图缓存，先不删除
		// localAppData := os.Getenv("LOCALAPPDATA")
		// if localAppData != "" {
		// 	thumbCache := filepath.Join(localAppData, "Microsoft", "Windows", "Explorer")
		// 	targets = append(targets, CleanupTarget{
		// 		ID:   "thumb_cache",
		// 		Name: "缩略图缓存",
		// 		Path: thumbCache,
		// 		Desc: "存储文件夹中图片和文件的预览图缓存",
		// 	})
		// }

		// 回收站 - 每个盘符的 $Recycle.Bin
		for c := 'A'; c <= 'Z'; c++ {
			drive := string(c) + `:\$Recycle.Bin`
			if _, err := os.Stat(drive); err == nil {
				targets = append(targets, CleanupTarget{
					ID:   fmt.Sprintf("recycle_%c", c),
					Name: fmt.Sprintf("回收站 (%c:)", c),
					Path: drive,
					Desc: "回收站在硬盘上的实际位置，清空即删除",
				})
			}
		}

	case isDarwin:
		// 用户临时文件
		targets = append(targets, CleanupTarget{
			ID:   "user_temp",
			Name: "用户临时文件",
			Path: os.TempDir(),
			Desc: "当前用户产生的临时文件 (/tmp 或 TMPDIR)",
		})

		// 系统临时文件
		targets = append(targets, CleanupTarget{
			ID:   "sys_temp",
			Name: "系统临时文件",
			Path: "/var/tmp",
			Desc: "系统运行产生的临时文件",
		})

		// 用户缓存
		home := os.Getenv("HOME")
		if home != "" {
			userCache := filepath.Join(home, "Library", "Caches")
			targets = append(targets, CleanupTarget{
				ID:   "user_cache",
				Name: "用户缓存",
				Path: userCache,
				Desc: "应用程序缓存文件，删除后会自动重建",
			})

			// 系统日志
			targets = append(targets, CleanupTarget{
				ID:   "sys_logs",
				Name: "系统日志",
				Path: filepath.Join(home, "Library", "Logs"),
				Desc: "应用程序和系统日志文件",
			})

			// Xcode DerivedData（开发者）
			derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
			if _, err := os.Stat(derivedData); err == nil {
				targets = append(targets, CleanupTarget{
					ID:   "xcode_derived",
					Name: "Xcode 编译缓存",
					Path: derivedData,
					Desc: "Xcode 编译产生的中间文件，可安全删除",
				})
			}
		}

	default:
		// Linux
		targets = append(targets, CleanupTarget{
			ID:   "user_temp",
			Name: "用户临时文件",
			Path: os.TempDir(),
			Desc: "当前用户产生的临时文件",
		})

		home := os.Getenv("HOME")
		if home != "" {
			targets = append(targets, CleanupTarget{
				ID:   "user_cache",
				Name: "用户缓存",
				Path: filepath.Join(home, ".cache"),
				Desc: "应用程序缓存文件",
			})
		}
	}

	return targets
}

// scanPath 计算目录大小和文件数
func scanPath(path string) (int64, int) {
	var size int64
	var count int
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size += info.Size()
		count++
		return nil
	})
	return size, count
}

// handleCleanupScan 扫描清理目标，返回各项大小和文件数
func handleCleanupScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	targets := getCleanupTargets()
	if len(targets) == 0 {
		json.NewEncoder(w).Encode(map[string]any{
			"targets": []CleanupTarget{},
			"message": "当前系统暂不支持清理功能",
		})
		return
	}

	// 并发扫描
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			size, count := scanPath(targets[idx].Path)
			targets[idx].Size = size
			targets[idx].Count = count
		}(i)
	}
	wg.Wait()

	// 计算总大小
	var totalSize int64
	var totalCount int
	for _, t := range targets {
		totalSize += t.Size
		totalCount += t.Count
	}

	json.NewEncoder(w).Encode(map[string]any{
		"targets":    targets,
		"totalSize":  totalSize,
		"totalCount": totalCount,
	})
}

// handleCleanupDelete 删除选中的清理目标
func handleCleanupDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method != "POST" {
		json.NewEncoder(w).Encode(map[string]any{"error": "仅支持 POST"})
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}

	if len(req.IDs) == 0 {
		json.NewEncoder(w).Encode(map[string]any{"error": "未选择任何清理项"})
		return
	}

	targets := getCleanupTargets()
	targetMap := make(map[string]CleanupTarget)
	for _, t := range targets {
		targetMap[t.ID] = t
	}

	type result struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		Count int    `json:"count"`
		Error string `json:"error,omitempty"`
	}

	var results []result
	var totalDeleted int64
	var totalFiles int

	for _, id := range req.IDs {
		t, ok := targetMap[id]
		if !ok {
			results = append(results, result{ID: id, Name: "未知", Error: "目标不存在"})
			continue
		}

		size, count := scanPath(t.Path)
		err := cleanDir(t.Path)

		r := result{ID: id, Name: t.Name, Size: size, Count: count}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)

		if err == nil {
			totalDeleted += size
			totalFiles += count
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"results":      results,
		"totalDeleted": totalDeleted,
		"totalFiles":   totalFiles,
		"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

// cleanDir 遍历目录内部，先删文件再删空目录，保留顶层目录本身。
func cleanDir(path string) error {
	// 先收集所有路径（文件优先，目录倒序删除）
	var files []string
	var dirs []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if p == path {
			return nil // 跳过顶层目录本身
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return err
	}

	var errs []string
	skipped := 0 // 程序占用或无法删除的文件数

	// 先删除所有文件
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			// 忽略不存在的文件
			if !os.IsNotExist(err) {
				skipped++
				if devMode {
					errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(f), err))
				}
			}
		}
	}

	// 再倒序删除空目录（最深层的先删）
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil {
			// 目录非空或权限不足，忽略不存在的
			if !os.IsNotExist(err) {
				skipped++
				if devMode {
					errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(dirs[i]), err))
				}
			}
		}
	}

	if skipped > 0 {
		msg := fmt.Sprintf("程序正在占用或想要手动清理（共 %d 项跳过）", skipped)
		if devMode && len(errs) > 0 {
			msg += ": " + strings.Join(errs, "; ")
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
