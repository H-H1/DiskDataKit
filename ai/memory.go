package ai

import "sync"

// Memory 对话记忆管理，按会话 ID 存储上下文消息。
type Memory struct {
	mu         sync.RWMutex
	sessions   map[string][]Message
	maxHistory int // 每个会话最多保留的消息数（不含 system）
}

// NewMemory 创建记忆管理器。
// maxHistory 为每个会话保留的最大消息数（0 表示不限制）。
func NewMemory(maxHistory int) *Memory {
	if maxHistory <= 0 {
		maxHistory = 20
	}
	return &Memory{
		sessions:   make(map[string][]Message),
		maxHistory: maxHistory,
	}
}

// Add 向指定会话追加消息。
func (m *Memory) Add(sessionID string, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[sessionID] = append(m.sessions[sessionID], msg)

	// 超过上限时，保留最新的消息（FIFO 淘汰）
	if len(m.sessions[sessionID]) > m.maxHistory {
		m.sessions[sessionID] = m.sessions[sessionID][len(m.sessions[sessionID])-m.maxHistory:]
	}
}

// Get 获取指定会话的所有消息（不含 system，system 由调用方单独管理）。
func (m *Memory) Get(sessionID string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msgs := m.sessions[sessionID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

// GetWithSystem 获取带 system 消息的完整上下文。
func (m *Memory) GetWithSystem(sessionID string, systemPrompt string) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Message
	if systemPrompt != "" {
		result = append(result, Message{Role: RoleSystem, Content: systemPrompt})
	}
	result = append(result, m.sessions[sessionID]...)
	return result
}

// Clear 清除指定会话的记忆。
func (m *Memory) Clear(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// ClearAll 清除所有会话记忆。
func (m *Memory) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string][]Message)
}

// Count 返回指定会话的消息数量。
func (m *Memory) Count(sessionID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions[sessionID])
}

// Sessions 返回所有活跃的会话 ID。
func (m *Memory) Sessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}
