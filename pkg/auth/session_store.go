// session_store.go 实现用户会话管理，包括会话创建、查询、撤销和并发数限制。
// 基于内存存储实现，适用于单实例部署。多实例场景建议替换为 Redis 实现。

package auth

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session 用户会话信息
type Session struct {
	ID        string    `json:"id"`         // 会话唯一标识
	UserID    string    `json:"user_id"`    // 用户 ID
	IP        string    `json:"ip"`         // 登录 IP
	UserAgent string    `json:"user_agent"` // 浏览器 UA
	CreatedAt time.Time `json:"created_at"` // 创建时间
	LastSeen  time.Time `json:"last_seen"`  // 最后活跃时间
}

// SessionStore 会话存储。
// 使用内存 map 实现，适用于单实例部署。多实例场景建议替换为 Redis 实现。
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // key: sessionID
	userIdx  map[string][]string // key: userID, value: sessionID 列表（索引加速查询）
}

// NewSessionStore 创建一个新的会话存储实例
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		userIdx:  make(map[string][]string),
	}
}

// Create 创建一个新的会话。
// 如果 Session.ID 为空，自动生成 UUID 作为会话标识。
func (s *SessionStore) Create(_ context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session 不能为 nil")
	}
	if session.UserID == "" {
		return fmt.Errorf("session.UserID 不能为空")
	}

	// 自动生成会话 ID
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	now := time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.LastSeen.IsZero() {
		session.LastSeen = now
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session
	s.userIdx[session.UserID] = append(s.userIdx[session.UserID], session.ID)

	return nil
}

// Get 根据会话 ID 获取会话信息
func (s *SessionStore) Get(_ context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}
	return session, nil
}

// ListByUser 列出指定用户的所有活跃会话，按创建时间倒序排列
func (s *SessionStore) ListByUser(_ context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID 不能为空")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs, ok := s.userIdx[userID]
	if !ok {
		return nil, nil
	}

	var result []*Session
	for _, sid := range sessionIDs {
		if session, exists := s.sessions[sid]; exists {
			result = append(result, session)
		}
	}

	// 按创建时间倒序排列（最新的在前）
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// Revoke 撤销（踢出）指定会话
func (s *SessionStore) Revoke(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	// 从用户索引中移除
	s.removeFromUserIndex(session.UserID, sessionID)
	// 删除会话
	delete(s.sessions, sessionID)

	return nil
}

// RevokeAll 撤销（踢出）用户的所有会话
func (s *SessionStore) RevokeAll(_ context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sessionIDs, ok := s.userIdx[userID]
	if !ok {
		return nil
	}

	// 删除所有会话
	for _, sid := range sessionIDs {
		delete(s.sessions, sid)
	}

	// 清除用户索引
	delete(s.userIdx, userID)

	return nil
}

// EnforceMaxConcurrent 强制执行最大并发会话数限制。
// 当用户的活跃会话数超过上限时，踢出最旧的会话直至满足限制。
func (s *SessionStore) EnforceMaxConcurrent(_ context.Context, userID string, max int) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}
	if max <= 0 {
		return fmt.Errorf("最大并发数必须大于 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sessionIDs, ok := s.userIdx[userID]
	if !ok || len(sessionIDs) <= max {
		return nil
	}

	// 收集用户所有有效会话
	var sessions []*Session
	for _, sid := range sessionIDs {
		if session, exists := s.sessions[sid]; exists {
			sessions = append(sessions, session)
		}
	}

	// 按创建时间升序排列（最旧的在前）
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})

	// 踢出超出上限的最旧会话
	removeCount := len(sessions) - max
	for i := 0; i < removeCount; i++ {
		delete(s.sessions, sessions[i].ID)
		s.removeFromUserIndex(userID, sessions[i].ID)
	}

	return nil
}

// Touch 更新会话的最后活跃时间
func (s *SessionStore) Touch(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	session.LastSeen = time.Now()
	return nil
}

// removeFromUserIndex 从用户索引中移除指定的会话 ID（内部方法，调用者须持有锁）
func (s *SessionStore) removeFromUserIndex(userID, sessionID string) {
	ids := s.userIdx[userID]
	for i, id := range ids {
		if id == sessionID {
			s.userIdx[userID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	// 如果用户没有会话了，清除索引条目
	if len(s.userIdx[userID]) == 0 {
		delete(s.userIdx, userID)
	}
}
