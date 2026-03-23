// session_store_test.go 测试会话管理的核心功能：
// - 会话创建和查询
// - 会话撤销
// - 并发会话数限制

package auth

import (
	"context"
	"testing"
	"time"
)

// TestSessionCreate 测试创建会话
func TestSessionCreate(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	session := &Session{
		UserID:    "user-001",
		IP:        "1.2.3.4",
		UserAgent: "Mozilla/5.0",
	}

	if err := store.Create(ctx, session); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	// 应自动生成 ID
	if session.ID == "" {
		t.Error("应自动生成会话 ID")
	}

	// 应设置创建时间
	if session.CreatedAt.IsZero() {
		t.Error("应自动设置创建时间")
	}
}

// TestSessionCreateNilSession 测试创建空会话
func TestSessionCreateNilSession(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	if err := store.Create(ctx, nil); err == nil {
		t.Error("nil session 应返回错误")
	}
}

// TestSessionCreateEmptyUserID 测试创建无用户 ID 的会话
func TestSessionCreateEmptyUserID(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	session := &Session{IP: "1.2.3.4"}
	if err := store.Create(ctx, session); err == nil {
		t.Error("空 UserID 应返回错误")
	}
}

// TestSessionGet 测试获取会话
func TestSessionGet(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	session := &Session{
		UserID:    "user-001",
		IP:        "1.2.3.4",
		UserAgent: "Mozilla/5.0",
	}
	store.Create(ctx, session)

	got, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if got.UserID != "user-001" {
		t.Errorf("UserID 期望 user-001，实际 %s", got.UserID)
	}
}

// TestSessionGetNotFound 测试获取不存在的会话
func TestSessionGetNotFound(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "non-existent")
	if err == nil {
		t.Error("不存在的会话应返回错误")
	}
}

// TestSessionListByUser 测试列出用户会话
func TestSessionListByUser(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	// 创建多个会话
	for i := 0; i < 3; i++ {
		session := &Session{
			UserID:    "user-001",
			IP:        "1.2.3.4",
			UserAgent: "Mozilla/5.0",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		store.Create(ctx, session)
	}

	// 创建其他用户的会话
	store.Create(ctx, &Session{UserID: "user-002", IP: "5.6.7.8"})

	sessions, err := store.ListByUser(ctx, "user-001")
	if err != nil {
		t.Fatalf("ListByUser 失败: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("期望 3 个会话，实际 %d 个", len(sessions))
	}

	// 应按创建时间倒序排列
	for i := 1; i < len(sessions); i++ {
		if sessions[i].CreatedAt.After(sessions[i-1].CreatedAt) {
			t.Error("会话应按创建时间倒序排列")
		}
	}
}

// TestSessionListByUserEmpty 测试列出无会话的用户
func TestSessionListByUserEmpty(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	sessions, err := store.ListByUser(ctx, "no-such-user")
	if err != nil {
		t.Fatalf("ListByUser 失败: %v", err)
	}

	if sessions != nil {
		t.Errorf("无会话时应返回 nil，实际 %v", sessions)
	}
}

// TestSessionRevoke 测试撤销单个会话
func TestSessionRevoke(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	session := &Session{UserID: "user-001", IP: "1.2.3.4"}
	store.Create(ctx, session)

	if err := store.Revoke(ctx, session.ID); err != nil {
		t.Fatalf("Revoke 失败: %v", err)
	}

	// 撤销后应无法获取
	_, err := store.Get(ctx, session.ID)
	if err == nil {
		t.Error("已撤销的会话不应可获取")
	}
}

// TestSessionRevokeNotFound 测试撤销不存在的会话
func TestSessionRevokeNotFound(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	if err := store.Revoke(ctx, "non-existent"); err == nil {
		t.Error("撤销不存在的会话应返回错误")
	}
}

// TestSessionRevokeAll 测试撤销用户所有会话
func TestSessionRevokeAll(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	// 创建多个会话
	for i := 0; i < 3; i++ {
		store.Create(ctx, &Session{UserID: "user-001", IP: "1.2.3.4"})
	}
	store.Create(ctx, &Session{UserID: "user-002", IP: "5.6.7.8"})

	if err := store.RevokeAll(ctx, "user-001"); err != nil {
		t.Fatalf("RevokeAll 失败: %v", err)
	}

	// user-001 应无会话
	sessions, _ := store.ListByUser(ctx, "user-001")
	if len(sessions) != 0 {
		t.Errorf("RevokeAll 后应无会话，实际 %d 个", len(sessions))
	}

	// user-002 的会话不应受影响
	sessions, _ = store.ListByUser(ctx, "user-002")
	if len(sessions) != 1 {
		t.Errorf("user-002 应有 1 个会话，实际 %d 个", len(sessions))
	}
}

// TestSessionEnforceMaxConcurrent 测试并发会话数限制
func TestSessionEnforceMaxConcurrent(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	// 创建 5 个会话
	var sessionIDs []string
	for i := 0; i < 5; i++ {
		s := &Session{
			UserID:    "user-001",
			IP:        "1.2.3.4",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		store.Create(ctx, s)
		sessionIDs = append(sessionIDs, s.ID)
	}

	// 限制最多 3 个并发
	if err := store.EnforceMaxConcurrent(ctx, "user-001", 3); err != nil {
		t.Fatalf("EnforceMaxConcurrent 失败: %v", err)
	}

	sessions, _ := store.ListByUser(ctx, "user-001")
	if len(sessions) != 3 {
		t.Errorf("期望 3 个会话，实际 %d 个", len(sessions))
	}

	// 被踢出的应是最旧的 2 个会话
	_, err1 := store.Get(ctx, sessionIDs[0])
	_, err2 := store.Get(ctx, sessionIDs[1])
	if err1 == nil || err2 == nil {
		t.Error("最旧的 2 个会话应被踢出")
	}

	// 最新的 3 个会话应保留
	for _, id := range sessionIDs[2:] {
		if _, err := store.Get(ctx, id); err != nil {
			t.Errorf("会话 %s 应保留，但获取失败: %v", id, err)
		}
	}
}

// TestSessionEnforceMaxConcurrentNoAction 测试未超限时不执行踢出
func TestSessionEnforceMaxConcurrentNoAction(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	store.Create(ctx, &Session{UserID: "user-001", IP: "1.2.3.4"})
	store.Create(ctx, &Session{UserID: "user-001", IP: "5.6.7.8"})

	// 限制 5 个，当前只有 2 个，不应踢出
	if err := store.EnforceMaxConcurrent(ctx, "user-001", 5); err != nil {
		t.Fatalf("EnforceMaxConcurrent 失败: %v", err)
	}

	sessions, _ := store.ListByUser(ctx, "user-001")
	if len(sessions) != 2 {
		t.Errorf("未超限时不应踢出，期望 2 个会话，实际 %d 个", len(sessions))
	}
}

// TestSessionTouch 测试更新最后活跃时间
func TestSessionTouch(t *testing.T) {
	store := NewSessionStore()
	ctx := context.Background()

	session := &Session{
		UserID:    "user-001",
		IP:        "1.2.3.4",
		LastSeen:  time.Now().Add(-1 * time.Hour),
	}
	store.Create(ctx, session)

	oldLastSeen := session.LastSeen

	// 等一小段时间确保时间变化
	time.Sleep(1 * time.Millisecond)

	if err := store.Touch(ctx, session.ID); err != nil {
		t.Fatalf("Touch 失败: %v", err)
	}

	got, _ := store.Get(ctx, session.ID)
	if !got.LastSeen.After(oldLastSeen) {
		t.Error("Touch 应更新 LastSeen 时间")
	}
}
