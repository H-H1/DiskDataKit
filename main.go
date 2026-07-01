package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

// FileItem 描述单个文件或目录的信息。
type FileItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Ext     string    `json:"ext"`
}

// listResponse 是 /api/files 接口的返回结构。
type listResponse struct {
	Path   string     `json:"path"`
	Parent string     `json:"parent"`
	Items  []FileItem `json:"items"`
	IsRoot bool       `json:"isRoot"`
	Err    string     `json:"error,omitempty"`
}

// 缓存当前系统信息，避免反复调用 runtime.GOOS
var (
	goos      = runtime.GOOS
	isWindows = goos == "windows"
	isDarwin  = goos == "darwin"
	isLinux   = goos == "linux"
)

// 文件大小缓存管理器，启动时加载，运行时更新。
var sizeCache *SizeCache

// 最近访问文件夹记录，启动时加载，选择时更新。
var recentFolders *RecentStore

func main() {
	fmt.Printf("DiskDataKit 已启动 | 系统: %s\n", goos)
	var err error
	sizeCache, err = NewSizeCache(cacheFilePath())
	if err != nil {
		log.Printf("缓存初始化失败: %v", err)
		sizeCache = &SizeCache{Data: make(map[string]CacheData)}
	}
	recentFolders, err = NewRecentStore(recentFilePath())
	if err != nil {
		log.Printf("最近访问初始化失败: %v", err)
		recentFolders = &RecentStore{FilePath: recentFilePath(), Folders: []string{}}
	}

	// 将嵌入的 web 目录作为静态资源服务（禁用缓存，确保更新后立即生效）
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("无法加载嵌入的前端资源: %v", err)
	}
	http.Handle("/", noCache(http.FileServer(http.FS(webContent))))
	http.HandleFunc("/api/files", handleListFiles)
	http.HandleFunc("/api/drives", handleListDrives)
	http.HandleFunc("/api/size", handleSize)
	http.HandleFunc("/api/recent", handleRecent)
	http.HandleFunc("/api/pickFolder", handlePickFolder)

	addr := ":8080"
	url := "http://localhost" + addr
	fmt.Printf("访问 %s 查看文件管理界面\n", url)
	go openBrowser(url)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// noCache 包装 handler，添加禁止缓存的响应头。
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}

// openBrowser 调用系统默认浏览器打开指定地址。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch {
	case isWindows:
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case isDarwin:
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// cacheFilePath 返回缓存文件路径，按系统区分缓存位置。
func cacheFilePath() string {
	var dir string
	switch {
	case isWindows:
		// Windows: 使用 LocalAppData 目录
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "DiskDataKit")
	case isDarwin:
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Caches", "DiskDataKit")
	default:
		dir = os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = filepath.Join(os.Getenv("HOME"), ".cache")
		}
		dir = filepath.Join(dir, "DiskDataKit")
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	return filepath.Join(dir, "size_cache.json")
}

// recentFilePath 返回最近访问文件夹记录的文件路径。
func recentFilePath() string {
	return filepath.Join(filepath.Dir(cacheFilePath()), "recent_folders.json")
}

// CacheData 单条缓存数据，文件绝对路径作为 map key。
type CacheData struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// SizeCache 缓存管理器。
type SizeCache struct {
	mu        sync.RWMutex
	CacheFile string
	Data      map[string]CacheData
	dirty     bool
}

// NewSizeCache 创建缓存管理器并加载已有数据。
func NewSizeCache(cachePath string) (*SizeCache, error) {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %v", err)
	}
	sc := &SizeCache{
		CacheFile: cachePath,
		Data:      make(map[string]CacheData),
	}
	if _, err := os.Stat(cachePath); err == nil {
		if err := sc.Load(); err != nil {
			log.Printf("加载缓存失败: %v，将创建新缓存", err)
		}
	}
	return sc, nil
}

// Load 从 JSON 文件加载缓存。
func (sc *SizeCache) Load() error {
	f, err := os.Open(sc.CacheFile)
	if err != nil {
		return err
	}
	defer f.Close()

	sc.mu.Lock()
	defer sc.mu.Unlock()
	return json.NewDecoder(f).Decode(&sc.Data)
}

// Save 全量覆盖保存缓存为 JSON 文件。
func (sc *SizeCache) Save() error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	f, err := os.Create(sc.CacheFile)
	if err != nil {
		return fmt.Errorf("创建缓存文件失败: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(sc.Data)
}

// Get 读取缓存。
func (sc *SizeCache) Get(path string) (CacheData, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	d, ok := sc.Data[path]
	return d, ok
}

// Set 仅更新内存缓存，标记 dirty，不触发磁盘写入。
func (sc *SizeCache) Set(path string, size int64, modTime time.Time) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// 已存在且大小未变、修改时间未变 → 跳过
	if old, ok := sc.Data[path]; ok && old.Size == size && old.ModTime.Equal(modTime) {
		return
	}
	sc.Data[path] = CacheData{
		Size:    size,
		ModTime: modTime,
	}
	sc.dirty = true
}

// Flush 若有未保存的变更，则全量保存一次到文件。
func (sc *SizeCache) Flush() {
	sc.mu.RLock()
	if !sc.dirty {
		sc.mu.RUnlock()
		return
	}
	sc.mu.RUnlock()

	if err := sc.Save(); err != nil {
		log.Printf("缓存保存失败: %v", err)
	}
	sc.mu.Lock()
	sc.dirty = false
	sc.mu.Unlock()
}

// RecentStore 最近访问文件夹记录管理器。
type RecentStore struct {
	mu       sync.Mutex
	FilePath string
	Folders  []string
	dirty    bool
}

const maxRecent = 10

// NewRecentStore 创建最近访问记录管理器并加载已有数据。
func NewRecentStore(path string) (*RecentStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建最近访问目录失败: %v", err)
	}
	rs := &RecentStore{
		FilePath: path,
		Folders:  []string{},
	}
	if _, err := os.Stat(path); err == nil {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&rs.Folders); err != nil {
			log.Printf("加载最近访问失败: %v，将创建新记录", err)
		}
	}
	return rs, nil
}

// Add 添加一条最近访问记录（去重、置顶、保留 maxRecent 条）。
func (rs *RecentStore) Add(path string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	// 去重：已存在则移除旧位置
	for i, f := range rs.Folders {
		if strings.EqualFold(f, path) {
			rs.Folders = append(rs.Folders[:i], rs.Folders[i+1:]...)
			break
		}
	}
	// 置顶
	rs.Folders = append([]string{path}, rs.Folders...)
	// 限制数量
	if len(rs.Folders) > maxRecent {
		rs.Folders = rs.Folders[:maxRecent]
	}
	rs.dirty = true
}

// List 返回最近访问文件夹列表的副本。
func (rs *RecentStore) List() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make([]string, len(rs.Folders))
	copy(out, rs.Folders)
	return out
}

// Save 若有变更则保存到文件。
func (rs *RecentStore) Save() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if !rs.dirty {
		return nil
	}
	f, err := os.Create(rs.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(rs.Folders); err != nil {
		return err
	}
	rs.dirty = false
	return nil
}

// cachedFileSize 读取文件大小，若缓存命中且文件未修改则直接返回缓存结果。
func cachedFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("路径不是文件: %s", path)
	}

	if d, ok := sizeCache.Get(path); ok && d.ModTime.Equal(info.ModTime()) {
		return d.Size, nil
	}

	size := info.Size()
	sizeCache.Set(path, size, info.ModTime())
	return size, nil
}

// handleSize 异步计算单个路径的大小（前端并发请求）。
// 文件和文件夹大小都缓存，修改时间未变则直接返回缓存值。
func handleSize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	path := r.URL.Query().Get("path")
	if path == "" {
		json.NewEncoder(w).Encode(map[string]any{"path": "", "size": 0, "error": "缺少 path 参数"})
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"path": path, "size": 0, "error": err.Error()})
		return
	}

	info, err := os.Stat(abs)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"path": abs, "size": 0, "error": err.Error()})
		return
	}

	// 文件和文件夹都走缓存：命中且修改时间一致直接返回
	if d, ok := sizeCache.Get(abs); ok && d.ModTime.Equal(info.ModTime()) {
		json.NewEncoder(w).Encode(map[string]any{"path": abs, "size": d.Size})
		return
	}

	var size int64
	if info.IsDir() {
		size = dirSize(abs)
	} else {
		size = info.Size()
	}
	sizeCache.Set(abs, size, info.ModTime())
	// 每个请求只保存一次全量缓存
	sizeCache.Flush()

	json.NewEncoder(w).Encode(map[string]any{"path": abs, "size": size})
}

// defaultPath 返回前端启动默认路径（用户主目录）。
func defaultPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "/"
}

// handleListFiles 列出指定路径下的文件与目录。
func handleListFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	path := r.URL.Query().Get("path")
	if path == "" {
		path = defaultPath()
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		writeErr(w, path, err)
		return
	}

	info, err := os.Stat(abs)
	if err != nil {
		writeErr(w, abs, err)
		return
	}
	if !info.IsDir() {
		writeErr(w, abs, fmt.Errorf("路径不是目录: %s", abs))
		return
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		// 无权限访问时返回空列表，而不是报错中断
		resp := listResponse{
			Path:   abs,
			Parent: parentPath(abs),
			Items:  []FileItem{},
			IsRoot: isRoot(abs),
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	items := make([]FileItem, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		full := filepath.Join(abs, e.Name())
		items = append(items, FileItem{
			Name:    e.Name(),
			Path:    full,
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Ext:     fileExt(e.Name(), e.IsDir()),
		})
	}

	// 目录优先，再按名称排序
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	resp := listResponse{
		Path:   abs,
		Parent: parentPath(abs),
		Items:  items,
		IsRoot: isRoot(abs),
	}
	json.NewEncoder(w).Encode(resp)
}

// handleListDrives 返回可用的磁盘根路径（Windows 上为盘符列表）。
func handleListDrives(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	drives := listRoots()
	json.NewEncoder(w).Encode(drives)
}

// handleRecent 处理最近访问文件夹的 GET（列表）和 POST（添加）。
func handleRecent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]any{"folders": recentFolders.List()})
	case http.MethodPost:
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		if body.Path == "" {
			json.NewEncoder(w).Encode(map[string]any{"error": "缺少 path 参数"})
			return
		}
		abs, err := filepath.Abs(body.Path)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		if _, err := os.Stat(abs); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		recentFolders.Add(abs)
		recentFolders.Save()
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handlePickFolder 调用系统原生对话框选择文件夹。
func handlePickFolder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	path, err := pickFolder()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"path": "", "error": err.Error()})
		return
	}
	if path != "" {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"path": path})
}

// listRoots 获取系统可用的根路径列表。
func listRoots() []string {
	if isWindows {
		var roots []string
		for c := 'A'; c <= 'Z'; c++ {
			drive := string(c) + `:\`
			if _, err := os.Stat(drive); err == nil {
				roots = append(roots, drive)
			}
		}
		return roots
	}
	// Unix-like 系统使用根目录
	if _, err := os.Stat("/"); err == nil {
		return []string{"/"}
	}
	return nil
}

// pickFolder 调用系统原生对话框让用户选择文件夹，返回所选路径（取消则返回空字符串）。
func pickFolder() (string, error) {
	switch {
	case isWindows:
		return pickFolderWindows()
	case isDarwin:
		return pickFolderDarwin()
	default:
		return pickFolderLinux()
	}
}

// pickFolderWindows 使用 PowerShell 调用 FolderBrowserDialog。
func pickFolderWindows() (string, error) {
	psScript := `Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = '选择文件夹'
$dialog.ShowNewFolderButton = $true
$form = New-Object System.Windows.Forms.Form
$form.TopMost = $true
$form.ShowInTaskbar = $false
$form.WindowState = 'Minimized'
$form.Show()
$form.Hide()
if ($dialog.ShowDialog($form) -eq 'OK') {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    Write-Output $dialog.SelectedPath
}
$form.Close()`
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法打开文件夹选择器: %v", err)
	}
	path := strings.TrimSpace(string(out))
	// 去除可能的 UTF-8 BOM
	path = strings.TrimPrefix(path, "\ufeff")
	return path, nil
}

// pickFolderDarwin 使用 osascript 调用 macOS 原生文件夹选择器。
func pickFolderDarwin() (string, error) {
	cmd := exec.Command("osascript", "-e", `set chosenFolder to choose folder
return POSIX path of chosenFolder`)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法打开文件夹选择器: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// pickFolderLinux 使用 zenity 调用文件夹选择器。
func pickFolderLinux() (string, error) {
	cmd := exec.Command("zenity", "--file-selection", "--directory", "--title=选择文件夹")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("无法打开文件夹选择器（需要安装 zenity）: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parentPath 返回上级目录路径，到达根时返回自身。
func parentPath(p string) string {
	parent := filepath.Dir(p)
	if parent == p {
		return ""
	}
	return parent
}

// isRoot 判断路径是否已到达根/盘符根。
func isRoot(p string) bool {
	parent := filepath.Dir(p)
	return parent == p
}

// fileExt 返回文件扩展名（小写，不含点）。
func fileExt(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	return strings.TrimPrefix(ext, ".")
}

// dirSize 使用 filepath.WalkDir 遍历目录，累加所有文件大小。
// 每个文件通过 os.Stat() 获取大小并缓存，文件夹大小不单独缓存。
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		// 通过 os.Stat() 获取文件大小并缓存
		size, err := cachedFileSize(p)
		if err != nil {
			return nil
		}
		total += size
		return nil
	})
	return total
}

// writeErr 在出错时返回包含错误信息的响应。
func writeErr(w http.ResponseWriter, path string, err error) {
	json.NewEncoder(w).Encode(listResponse{
		Path: path,
		Err:  err.Error(),
	})
}
