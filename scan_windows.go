//go:build windows

package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// scanAll 扫描已安装程序、运行中进程、可疑文件、启动项四类。
func scanAll() []ScanItem {
	var items []ScanItem
	items = append(items, scanInstalled()...)
	items = append(items, scanProcesses()...)
	items = append(items, scanSuspiciousFiles()...)
	items = append(items, scanStartup()...)
	return items
}

// scanInstalled 读取注册表 Uninstall 键，列出已安装程序。
func scanInstalled() []ScanItem {
	var items []ScanItem
	uninstallPaths := []struct {
		root  registry.Key
		label string
		path  string
	}{
		{registry.LOCAL_MACHINE, "HKLM", `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, "HKLM", `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, "HKCU", `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	for _, up := range uninstallPaths {
		k, err := registry.OpenKey(up.root, up.path, registry.READ)
		if err != nil {
			continue
		}
		subs, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, sub := range subs {
			full := up.path + `\` + sub
			sk, err := registry.OpenKey(up.root, full, registry.READ)
			if err != nil {
				continue
			}
			name, _, _ := sk.GetStringValue("DisplayName")
			pub, _, _ := sk.GetStringValue("Publisher")
			loc, _, _ := sk.GetStringValue("InstallLocation")
			icon, _, _ := sk.GetStringValue("DisplayIcon")
			sk.Close()
			if name == "" {
				continue
			}
			path := loc
			if path == "" && icon != "" {
				path = strings.Split(icon, ",")[0]
				path = strings.Trim(path, `"`)
			}
			items = append(items, ScanItem{
				Category:  "installed",
				Name:      name,
				Path:      path,
				Publisher: pub,
				Location:  up.label + `\` + full,
			})
		}
	}
	return items
}

// scanProcesses 通过 PowerShell CIM 列出运行中进程及其路径。
func scanProcesses() []ScanItem {
	ps := `[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
@(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -ne $null } | Select-Object Name,ExecutablePath) | ConvertTo-Json -Compress`
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-Command", ps)
	setNoWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	out = trimBOM(out)
	var procs []struct {
		Name           string `json:"Name"`
		ExecutablePath string `json:"ExecutablePath"`
	}
	if err := json.Unmarshal(out, &procs); err != nil {
		return nil
	}
	items := make([]ScanItem, 0, len(procs))
	for _, p := range procs {
		if p.Name == "" {
			continue
		}
		items = append(items, ScanItem{
			Category: "process",
			Name:     p.Name,
			Path:     p.ExecutablePath,
			Location: "运行中进程",
		})
	}
	return items
}

// scanSuspiciousFiles 遍历 AppData/LocalAppData/ProgramData/Temp 查找可执行文件。
func scanSuspiciousFiles() []ScanItem {
	dirs := []string{
		os.Getenv("APPDATA"),
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("PROGRAMDATA"),
		os.Getenv("TEMP"),
	}
	var items []ScanItem
	const maxPerDir = 80
	const maxDepth = 4
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		count := 0
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				rel := strings.TrimPrefix(p, dir)
				rel = strings.Trim(rel, string(filepath.Separator))
				if rel != "" && strings.Count(rel, string(filepath.Separator)) >= maxDepth {
					return filepath.SkipDir
				}
				return nil
			}
			if count >= maxPerDir {
				return filepath.SkipDir
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".exe" || ext == ".scr" {
				items = append(items, ScanItem{
					Category: "file",
					Name:     d.Name(),
					Path:     p,
					Location: dir,
				})
				count++
			}
			return nil
		})
	}
	return items
}

// scanStartup 复用启动项数据，转为 ScanItem。
func scanStartup() []ScanItem {
	startups := listStartupItems()
	items := make([]ScanItem, 0, len(startups))
	for _, s := range startups {
		items = append(items, ScanItem{
			Category: "startup",
			Name:     s.Name,
			Path:     s.Path,
			Location: s.Location,
		})
	}
	return items
}

// trimBOM 去除 UTF-8 BOM。
func trimBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}
