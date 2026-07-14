package main

import (
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TrackChild 记录追踪文件夹下单个子项的快照。
type TrackChild struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// TrackEntry 一个被追踪的文件夹。
type TrackEntry struct {
	Path      string       `json:"path"`
	Name      string       `json:"name"`
	CreatedAt time.Time    `json:"createdAt"`
	Updated   time.Time    `json:"updated"`
	Children  []TrackChild `json:"children"`
}

// TrackStore 追踪文件夹管理器，持久化到 track_folders.bin。
type TrackStore struct {
	mu       sync.Mutex
	FilePath string
	Entries  map[string]*TrackEntry // key: 文件夹绝对路径
	dirty    bool
}

// NewTrackStore 创建追踪管理器并加载已有数据。
func NewTrackStore(path string) (*TrackStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	ts := &TrackStore{
		FilePath: path,
		Entries:  make(map[string]*TrackEntry),
	}
	if _, err := os.Stat(path); err == nil {
		if err := ts.Load(); err != nil {
			return nil, err
		}
	}
	return ts, nil
}

// Load 从 GZIP+Gob 文件加载。
func (ts *TrackStore) Load() error {
	f, err := os.Open(ts.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return gob.NewDecoder(gz).Decode(&ts.Entries)
}

// Save 全量保存。
func (ts *TrackStore) Save() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	f, err := os.Create(ts.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	return gob.NewEncoder(gz).Encode(ts.Entries)
}

// Flush 若有变更则保存。
func (ts *TrackStore) Flush() {
	ts.mu.Lock()
	dirty := ts.dirty
	ts.dirty = false
	ts.mu.Unlock()
	if dirty {
		_ = ts.Save()
	}
}

// List 返回所有追踪条目（按添加时间排序）。
func (ts *TrackStore) List() []*TrackEntry {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	result := make([]*TrackEntry, 0, len(ts.Entries))
	for _, e := range ts.Entries {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// Add 添加追踪文件夹并立即扫描快照。
func (ts *TrackStore) Add(path string) (*TrackEntry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errNotDir
	}

	children := scanTrackChildren(abs)

	ts.mu.Lock()
	entry := &TrackEntry{
		Path:      abs,
		Name:      filepath.Base(abs),
		CreatedAt: time.Now(),
		Updated:   time.Now(),
		Children:  children,
	}
	ts.Entries[abs] = entry
	ts.dirty = true
	ts.mu.Unlock()
	ts.Flush()
	return entry, nil
}

// Remove 取消追踪。
func (ts *TrackStore) Remove(path string) {
	abs, _ := filepath.Abs(path)
	ts.mu.Lock()
	delete(ts.Entries, abs)
	ts.dirty = true
	ts.mu.Unlock()
	ts.Flush()
}

// Refresh 重新扫描指定追踪文件夹，返回新旧对比结果。
func (ts *TrackStore) Refresh(path string) (*TrackDiff, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	old, ok := ts.Entries[abs]
	ts.mu.Unlock()
	if !ok {
		return nil, errNotTracked
	}

	newChildren := scanTrackChildren(abs)
	diff := computeDiff(old.Children, newChildren)

	ts.mu.Lock()
	if entry, ok := ts.Entries[abs]; ok {
		entry.Children = newChildren
		entry.Updated = time.Now()
		ts.dirty = true
	}
	ts.mu.Unlock()
	ts.Flush()

	return diff, nil
}

// scanTrackChildren 扫描目录的直接子项，计算大小。
func scanTrackChildren(dir string) []TrackChild {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []TrackChild{}
	}
	children := make([]TrackChild, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		// 不跳过隐藏文件，追踪完整内容
		info, err := e.Info()
		if err != nil {
			continue
		}
		full := filepath.Join(dir, name)
		child := TrackChild{
			Name:    name,
			Path:    full,
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		}
		if e.IsDir() {
			child.Size = dirSize(full)
		} else {
			child.Size = info.Size()
		}
		children = append(children, child)
	}
	return children
}

// TrackDiff 对比新旧快照的差异。
type TrackDiff struct {
	Path     string             `json:"path"`
	OldTotal int64              `json:"oldTotal"`
	NewTotal int64              `json:"newTotal"`
	Delta    int64              `json:"delta"`
	Added    []TrackChildDiff   `json:"added"`
	Removed  []TrackChildDiff   `json:"removed"`
	Changed  []TrackChildChange `json:"changed"`
}

// TrackChildDiff 新增或删除的子项。
type TrackChildDiff struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// TrackChildChange 大小发生变化的子项。
type TrackChildChange struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	OldSize int64  `json:"oldSize"`
	NewSize int64  `json:"newSize"`
	Delta   int64  `json:"delta"`
}

// computeDiff 对比新旧子项列表。
func computeDiff(old, new []TrackChild) *TrackDiff {
	oldMap := make(map[string]TrackChild, len(old))
	for _, c := range old {
		oldMap[c.Path] = c
	}
	newMap := make(map[string]TrackChild, len(new))
	for _, c := range new {
		newMap[c.Path] = c
	}

	var oldTotal, newTotal int64
	for _, c := range old {
		oldTotal += c.Size
	}
	for _, c := range new {
		newTotal += c.Size
	}

	diff := &TrackDiff{
		OldTotal: oldTotal,
		NewTotal: newTotal,
		Delta:    newTotal - oldTotal,
		Added:    []TrackChildDiff{},
		Removed:  []TrackChildDiff{},
		Changed:  []TrackChildChange{},
	}

	// 新增的
	for _, c := range new {
		if _, ok := oldMap[c.Path]; !ok {
			diff.Added = append(diff.Added, TrackChildDiff{
				Name: c.Name, Path: c.Path, IsDir: c.IsDir, Size: c.Size,
			})
		}
	}
	// 删除的
	for _, c := range old {
		if _, ok := newMap[c.Path]; !ok {
			diff.Removed = append(diff.Removed, TrackChildDiff{
				Name: c.Name, Path: c.Path, IsDir: c.IsDir, Size: c.Size,
			})
		}
	}
	// 大小变化的
	for _, c := range new {
		if o, ok := oldMap[c.Path]; ok && o.Size != c.Size {
			diff.Changed = append(diff.Changed, TrackChildChange{
				Name: c.Name, Path: c.Path, IsDir: c.IsDir,
				OldSize: o.Size, NewSize: c.Size, Delta: c.Size - o.Size,
			})
		}
	}

	return diff
}

var (
	errNotDir     = &trackError{"路径不是目录"}
	errNotTracked = &trackError{"该文件夹未被追踪"}
)

type trackError struct{ msg string }

func (e *trackError) Error() string { return e.msg }

// ---- HTTP Handlers ----

// handleTrackList 返回所有追踪的文件夹及其当前快照。
// GET /api/track/list
func handleTrackList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	entries := trackStore.List()
	// 同时计算每个追踪文件夹的新旧对比（如果用户想看实时差异）
	result := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		result = append(result, map[string]any{
			"path":       e.Path,
			"name":       e.Name,
			"createdAt":  e.CreatedAt,
			"updated":    e.Updated,
			"children":   e.Children,
			"childCount": len(e.Children),
		})
	}
	json.NewEncoder(w).Encode(map[string]any{
		"tracks": result,
	})
}

// handleTrackAdd 添加追踪文件夹。
// POST /api/track/add
// Body: {"path": "C:\\SomeFolder"}
func handleTrackAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Path == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "缺少 path 参数"})
		return
	}

	entry, err := trackStore.Add(req.Path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"track":   entry,
		"message": "已开始追踪: " + entry.Path,
	})
}

// handleTrackRemove 取消追踪。
// POST /api/track/remove
// Body: {"path": "C:\\SomeFolder"}
func handleTrackRemove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}
	trackStore.Remove(req.Path)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleTrackRefresh 重新扫描并返回对比差异。
// POST /api/track/refresh
// Body: {"path": "C:\\SomeFolder"}
func handleTrackRefresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Path == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "缺少 path 参数"})
		return
	}

	diff, err := trackStore.Refresh(req.Path)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"diff": diff,
	})
}
