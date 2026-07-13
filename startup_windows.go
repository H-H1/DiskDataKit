//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// 禁用项管理键：记录被禁用的 Explorer/IE 项的原始信息，便于恢复。
const mgmtKeyBase = `Software\DiskDataKit\Disabled`

// listStartupItems 读取 Windows 的登录启动项、资源管理器插件、IE 加载项，Ai分析。
func listStartupItems() []StartupItem {
	var items []StartupItem
	items = append(items, readLogon()...)
	items = append(items, readExplorer()...)
	items = append(items, readIE()...)
	return items
}

// ==================== Logon 登录启动项 ====================

// readLogon 读取注册表 Run/RunOnce 键与启动文件夹。
func readLogon() []StartupItem {
	var items []StartupItem
	runKeys := []struct {
		root  registry.Key
		label string
		path  string
	}{
		{registry.CURRENT_USER, "HKCU", `Software\Microsoft\Windows\CurrentVersion\Run`},
		{registry.LOCAL_MACHINE, "HKLM", `Software\Microsoft\Windows\CurrentVersion\Run`},
		{registry.CURRENT_USER, "HKCU", `Software\Microsoft\Windows\CurrentVersion\RunOnce`},
		{registry.LOCAL_MACHINE, "HKLM", `Software\Microsoft\Windows\CurrentVersion\RunOnce`},
	}
	for _, rk := range runKeys {
		items = append(items, readRunKey(rk.root, rk.label, rk.path)...)
	}
	items = append(items, readStartupFolder()...)
	return items
}

func readRunKey(root registry.Key, label, path string) []StartupItem {
	k, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadValueNames(-1)
	var items []StartupItem
	for _, name := range names {
		val, _, err := k.GetStringValue(name)
		if err != nil {
			continue
		}
		state := "enabled"
		if isRunDisabled(label, name) {
			state = "disabled"
		}
		items = append(items, StartupItem{
			Category: "logon",
			Name:     name,
			Path:     val,
			Location: label + `\` + path,
			State:    state,
		})
	}
	return items
}

func readStartupFolder() []StartupItem {
	var items []StartupItem
	folders := []string{
		os.Getenv("APPDATA") + `\Microsoft\Windows\Start Menu\Programs\Startup`,
		os.Getenv("PROGRAMDATA") + `\Microsoft\Windows\Start Menu\Programs\Startup`,
	}
	for _, folder := range folders {
		entries, err := os.ReadDir(folder)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			full := filepath.Join(folder, e.Name())
			state := "enabled"
			if isStartupFolderDisabled(full) {
				state = "disabled"
			}
			items = append(items, StartupItem{
				Category: "logon",
				Name:     e.Name(),
				Path:     full,
				Location: folder,
				State:    state,
			})
		}
	}
	return items
}

// isRunDisabled 通过 StartupApproved\Run 检测注册表启动项是否被禁用。
// 该机制与任务管理器一致：REG_BINARY 首字节 0x02=禁用，0x03=启用。
func isRunDisabled(label, name string) bool {
	root := registry.CURRENT_USER
	if label == "HKLM" {
		root = registry.LOCAL_MACHINE
	}
	k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue(name)
	if err != nil {
		return false
	}
	return len(b) >= 4 && b[0] == 0x02
}

func isStartupFolderDisabled(fullPath string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`, registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue(fullPath)
	if err != nil {
		return false
	}
	return len(b) >= 4 && b[0] == 0x02
}

// toggleLogon 启用/禁用登录启动项（写 StartupApproved 标志）。
func toggleLogon(item StartupItem, enable bool) error {
	flag := byte(0x03)
	if !enable {
		flag = 0x02
	}
	val := make([]byte, 12)
	val[0] = flag

	// 注册表 Run/RunOnce 项
	if strings.HasPrefix(item.Location, "HKCU") || strings.HasPrefix(item.Location, "HKLM") {
		root := registry.CURRENT_USER
		if strings.HasPrefix(item.Location, "HKLM") {
			root = registry.LOCAL_MACHINE
		}
		k, _, err := registry.CreateKey(root, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.SET_VALUE|registry.CREATE_SUB_KEY)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetBinaryValue(item.Name, val)
	}
	// 启动文件夹项
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`, registry.SET_VALUE|registry.CREATE_SUB_KEY)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(item.Path, val)
}

// ==================== Explorer 资源管理器插件 ====================

var explorerPaths = []string{
	`*\shellex\ContextMenuHandlers`,
	`Folder\shellex\ContextMenuHandlers`,
	`Directory\shellex\ContextMenuHandlers`,
	`Directory\Background\shellex\ContextMenuHandlers`,
	`AllFilesystemObjects\shellex\ContextMenuHandlers`,
}

func readExplorer() []StartupItem {
	var items []StartupItem
	for _, p := range explorerPaths {
		k, err := registry.OpenKey(registry.CLASSES_ROOT, p, registry.READ)
		if err != nil {
			continue
		}
		subs, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, sub := range subs {
			full := p + `\` + sub
			sk, err := registry.OpenKey(registry.CLASSES_ROOT, full, registry.READ)
			if err != nil {
				continue
			}
			clsid, _, _ := sk.GetStringValue("")
			sk.Close()
			state := "enabled"
			if isRegSubkeyDisabled("explorer", sub) {
				state = "disabled"
			}
			items = append(items, StartupItem{
				Category: "explorer",
				Name:     sub,
				Path:     clsid,
				Location: `HKCR\` + full,
				State:    state,
			})
		}
	}
	return items
}

// ==================== IE 加载项 ====================

var iePaths = []struct {
	label string
	path  string
}{
	{"Browser Helper Objects", `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\Browser Helper Objects`},
	{"Toolbar", `SOFTWARE\Microsoft\Internet Explorer\Toolbar`},
	{"Extensions", `SOFTWARE\Microsoft\Internet Explorer\Extensions`},
}

func readIE() []StartupItem {
	var items []StartupItem
	for _, ip := range iePaths {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, ip.path, registry.READ)
		if err != nil {
			continue
		}
		subs, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, sub := range subs {
			full := ip.path + `\` + sub
			sk, err := registry.OpenKey(registry.LOCAL_MACHINE, full, registry.READ)
			if err != nil {
				continue
			}
			desc, _, _ := sk.GetStringValue("")
			sk.Close()
			state := "enabled"
			if isRegSubkeyDisabled("ie", sub) {
				state = "disabled"
			}
			items = append(items, StartupItem{
				Category: "ie",
				Name:     sub,
				Path:     desc,
				Location: `HKLM\` + full,
				State:    state,
			})
		}
	}
	return items
}

// ==================== 通用注册表子键启用/禁用（Explorer/IE） ====================

// isRegSubkeyDisabled 检测某子键是否被本工具禁用（存在管理记录）。
func isRegSubkeyDisabled(cat, name string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, mgmtKeyBase+`\`+cat+`\`+name, registry.READ)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

// parseRegLocation 解析形如 HKCR\... / HKLM\... / HKCU\... 的位置字符串。
func parseRegLocation(loc string) (registry.Key, string, bool) {
	switch {
	case strings.HasPrefix(loc, `HKCR\`):
		return registry.CLASSES_ROOT, strings.TrimPrefix(loc, `HKCR\`), true
	case strings.HasPrefix(loc, `HKLM\`):
		return registry.LOCAL_MACHINE, strings.TrimPrefix(loc, `HKLM\`), true
	case strings.HasPrefix(loc, `HKCU\`):
		return registry.CURRENT_USER, strings.TrimPrefix(loc, `HKCU\`), true
	}
	return 0, "", false
}

// deleteRegTree 递归删除注册表子键（含其子键）。
func deleteRegTree(root registry.Key, path string) error {
	k, err := registry.OpenKey(root, path, registry.READ)
	if err == nil {
		subs, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, s := range subs {
			deleteRegTree(root, path+`\`+s)
		}
	}
	return registry.DeleteKey(root, path)
}

// disableRegSubkey 禁用：备份默认值到管理键，删除原子键。
func disableRegSubkey(cat string, item StartupItem) error {
	root, sub, ok := parseRegLocation(item.Location)
	if !ok {
		return fmt.Errorf("无法解析位置: %s", item.Location)
	}
	sk, err := registry.OpenKey(root, sub, registry.READ)
	if err != nil {
		return fmt.Errorf("打开注册表项失败: %v", err)
	}
	def, _, _ := sk.GetStringValue("")
	sk.Close()

	mgmt := mgmtKeyBase + `\` + cat + `\` + item.Name
	mk, _, err := registry.CreateKey(registry.CURRENT_USER, mgmt, registry.WRITE)
	if err != nil {
		return err
	}
	mk.SetStringValue("Default", def)
	mk.SetStringValue("Location", item.Location)
	mk.Close()

	return deleteRegTree(root, sub)
}

// enableRegSubkey 启用：从管理键恢复默认值，重建子键。
func enableRegSubkey(cat string, item StartupItem) error {
	mgmt := mgmtKeyBase + `\` + cat + `\` + item.Name
	mk, err := registry.OpenKey(registry.CURRENT_USER, mgmt, registry.READ)
	if err != nil {
		return fmt.Errorf("找不到禁用记录，可能已被外部恢复")
	}
	def, _, _ := mk.GetStringValue("Default")
	loc, _, _ := mk.GetStringValue("Location")
	mk.Close()

	root, sub, ok := parseRegLocation(loc)
	if !ok {
		return fmt.Errorf("无法解析位置: %s", loc)
	}
	sk, _, err := registry.CreateKey(root, sub, registry.WRITE)
	if err != nil {
		return err
	}
	if def != "" {
		sk.SetStringValue("", def)
	}
	sk.Close()

	deleteRegTree(registry.CURRENT_USER, mgmt)
	return nil
}

// resolveCLSIDToPath 通过 HKCR\CLSID\{clsid}\InprocServer32 解析 CLSID 对应的 DLL 路径。
func resolveCLSIDToPath(clsid string) (string, error) {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `CLSID\`+clsid+`\InprocServer32`, registry.READ)
	if err != nil {
		return "", err
	}
	defer k.Close()
	dll, _, err := k.GetStringValue("")
	if err != nil {
		return "", err
	}
	return dll, nil
}

// toggleStartupItem 启用/禁用指定启动项。
func toggleStartupItem(category, name, location string, enable bool) error {
	item := StartupItem{Category: category, Name: name, Location: location}
	switch category {
	case "logon":
		return toggleLogon(item, enable)
	case "explorer", "ie":
		if enable {
			return enableRegSubkey(category, item)
		}
		return disableRegSubkey(category, item)
	}
	return fmt.Errorf("未知类别: %s", category)
}
