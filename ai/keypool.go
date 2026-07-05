package ai

import (
	"sync"
	"time"
)

// KeyPool 多 API Key 池，支持轮询、健康检查、自动降级。
type KeyPool struct {
	mu      sync.Mutex
	keys    []*KeyInfo
	current int          // 轮询位置
	cooling map[int]time.Time // 冷却中的 key: index → 恢复时间
}

// NewKeyPool 创建 Key 池。
func NewKeyPool(keys ...string) *KeyPool {
	pool := &KeyPool{
		cooling: make(map[int]time.Time),
	}
	for _, k := range keys {
		pool.keys = append(pool.keys, &KeyInfo{
			Key:     k,
			Healthy: true,
		})
	}
	return pool
}

// Get 获取一个可用的 Key（轮询 + 跳过冷却中的）。
// 返回 Key 字符串和对应的 KeyInfo。如果没有可用 Key 返回空字符串。
func (p *KeyPool) Get() (string, *KeyInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.keys)
	if n == 0 {
		return "", nil
	}

	now := time.Now()
	// 检查冷却恢复
	for i, t := range p.cooling {
		if now.After(t) {
			delete(p.cooling, i)
			if p.keys[i] != nil {
				p.keys[i].Healthy = true
				p.keys[i].FailCount = 0
			}
		}
	}

	// 轮询查找可用 key
	for i := 0; i < n; i++ {
		idx := (p.current + i) % n
		info := p.keys[idx]
		if info != nil && info.Healthy {
			p.current = (idx + 1) % n
			info.LastUsed = now
			info.UseCount++
			return info.Key, info
		}
	}

	return "", nil
}

// MarkFailed 标记 Key 失败，连续失败 3 次进入冷却。
func (p *KeyPool) MarkFailed(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, info := range p.keys {
		if info != nil && info.Key == key {
			info.FailCount++
			if info.FailCount >= 3 {
				info.Healthy = false
				p.cooling[i] = time.Now().Add(60 * time.Second) // 冷却 60 秒
			}
			return
		}
	}
}

// MarkSuccess 标记 Key 成功，重置失败计数。
func (p *KeyPool) MarkSuccess(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, info := range p.keys {
		if info != nil && info.Key == key {
			info.FailCount = 0
			info.Healthy = true
			return
		}
	}
}

// Stats 返回所有 Key 的状态信息（不含 Key 本身）。
func (p *KeyPool) Stats() []*KeyInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]*KeyInfo, len(p.keys))
	for i, info := range p.keys {
		cp := *info
		result[i] = &cp
	}
	return result
}

// Count 返回 Key 总数。
func (p *KeyPool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.keys)
}

// Available 返回可用 Key 数量。
func (p *KeyPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	count := 0
	for i, info := range p.keys {
		if info == nil {
			continue
		}
		// 检查是否冷却恢复
		if t, ok := p.cooling[i]; ok && now.Before(t) {
			continue
		}
		if info.Healthy {
			count++
		}
	}
	return count
}
