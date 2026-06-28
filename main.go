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

func main() {
	// 将嵌入的 web 目录作为静态资源服务
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("无法加载嵌入的前端资源: %v", err)
	}
	http.Handle("/", http.FileServer(http.FS(webContent)))
	http.HandleFunc("/api/files", handleListFiles)
	http.HandleFunc("/api/drives", handleListDrives)

	addr := ":8080"
	url := "http://localhost" + addr
	fmt.Printf("DiskDataKit 已启动\n访问 %s 查看文件管理界面\n", url)
	go openBrowser(url)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// openBrowser 调用系统默认浏览器打开指定地址。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
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
		writeErr(w, abs, err)
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
	if runtime.GOOS == "windows" {
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

// writeErr 在出错时返回包含错误信息的响应。
func writeErr(w http.ResponseWriter, path string, err error) {
	json.NewEncoder(w).Encode(listResponse{
		Path: path,
		Err:  err.Error(),
	})
}
