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

func main() {
	fmt.Printf("DiskDataKit 已启动 | 系统: %s\n", goos)

	// 将嵌入的 web 目录作为静态资源服务（禁用缓存，确保更新后立即生效）
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("无法加载嵌入的前端资源: %v", err)
	}
	http.Handle("/", noCache(http.FileServer(http.FS(webContent))))
	http.HandleFunc("/api/files", handleListFiles)
	http.HandleFunc("/api/drives", handleListDrives)
	http.HandleFunc("/api/size", handleSize)

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

// handleSize 异步计算单个目录的总大小（前端并发请求）。
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
	size := dirSize(abs)
	json.NewEncoder(w).Encode(map[string]any{"path": abs, "size": size})
}

// handleListFiles 列出指定路径下的文件与目录。
func handleListFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	path := r.URL.Query().Get("path")
	if path == "" {
		path, _ = os.UserHomeDir()
		if path == "" {
			path, _ = os.Getwd()
		}
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

// dirSize 递归计算目录的总大小（字节），遇到错误跳过对应项。
func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
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
