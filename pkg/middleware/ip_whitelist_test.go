// ip_whitelist_test.go 测试 IP 白名单中间件的核心功能：
// - IPv4/IPv6 精确匹配
// - CIDR 网段匹配
// - 临时白名单及过期机制
// - X-Forwarded-For / X-Real-IP 头解析

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// okHandler 返回 200 OK 的测试处理器
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// TestIPWhitelistDisabled 测试白名单未启用时应放行所有请求
func TestIPWhitelistDisabled(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled: false,
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("白名单未启用时应放行，期望 200，实际 %d", rr.Code)
	}
}

// TestIPWhitelistExactIPv4 测试 IPv4 精确匹配
func TestIPWhitelistExactIPv4(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"192.168.1.100", "10.0.0.1"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"白名单内 IP", "192.168.1.100:8080", http.StatusOK},
		{"白名单内另一个 IP", "10.0.0.1:8080", http.StatusOK},
		{"白名单外 IP", "172.16.0.1:8080", http.StatusForbidden},
		{"未知 IP", "8.8.8.8:8080", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("IP %s: 期望 %d，实际 %d", tt.ip, tt.wantStatus, rr.Code)
			}
		})
	}
}

// TestIPWhitelistCIDR 测试 CIDR 网段匹配
func TestIPWhitelistCIDR(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"192.168.1.0/24", "10.0.0.0/8"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"192.168.1.x 网段内", "192.168.1.50:8080", http.StatusOK},
		{"192.168.1.x 网段边界", "192.168.1.255:8080", http.StatusOK},
		{"10.x.x.x 网段内", "10.100.200.50:8080", http.StatusOK},
		{"192.168.2.x 网段外", "192.168.2.1:8080", http.StatusForbidden},
		{"外部 IP", "8.8.8.8:8080", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("IP %s: 期望 %d，实际 %d", tt.ip, tt.wantStatus, rr.Code)
			}
		})
	}
}

// TestIPWhitelistIPv6 测试 IPv6 精确匹配
func TestIPWhitelistIPv6(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"::1", "fe80::1"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"IPv6 环回地址", "[::1]:8080", http.StatusOK},
		{"IPv6 链路本地地址", "[fe80::1]:8080", http.StatusOK},
		{"不在白名单的 IPv6", "[fe80::2]:8080", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("IP %s: 期望 %d，实际 %d", tt.ip, tt.wantStatus, rr.Code)
			}
		})
	}
}

// TestIPWhitelistTempAllow 测试临时白名单及过期机制
func TestIPWhitelistTempAllow(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled: true,
		TempAllow: []TempEntry{
			{CIDR: "172.16.0.1", ExpiresAt: time.Now().Add(1 * time.Hour)},    // 未过期
			{CIDR: "172.16.0.2", ExpiresAt: time.Now().Add(-1 * time.Hour)},   // 已过期
			{CIDR: "172.17.0.0/16", ExpiresAt: time.Now().Add(1 * time.Hour)}, // CIDR 临时白名单
		},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"临时白名单内（未过期）", "172.16.0.1:8080", http.StatusOK},
		{"临时白名单内（已过期）", "172.16.0.2:8080", http.StatusForbidden},
		{"临时 CIDR 白名单内", "172.17.1.1:8080", http.StatusOK},
		{"不在任何白名单中", "172.18.0.1:8080", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("IP %s: 期望 %d，实际 %d", tt.ip, tt.wantStatus, rr.Code)
			}
		})
	}
}

// TestIPWhitelistXForwardedFor 测试 X-Forwarded-For 头解析
func TestIPWhitelistXForwardedFor(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"10.0.0.1"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	// X-Forwarded-For 中第一个 IP 在白名单中
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1:8080" // 代理 IP
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("X-Forwarded-For 白名单内 IP 应放行，期望 200，实际 %d", rr.Code)
	}

	// X-Forwarded-For 中的 IP 不在白名单中
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "172.16.0.1:8080"
	req2.Header.Set("X-Forwarded-For", "8.8.8.8, 172.16.0.1")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("X-Forwarded-For 白名单外 IP 应拒绝，期望 403，实际 %d", rr2.Code)
	}
}

// TestIPWhitelistXRealIP 测试 X-Real-IP 头优先级高于 X-Forwarded-For
func TestIPWhitelistXRealIP(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"10.0.0.1"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	// X-Real-IP 设置为白名单内 IP
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1:8080"
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.Header.Set("X-Forwarded-For", "8.8.8.8") // 不在白名单中
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("X-Real-IP 应优先于 X-Forwarded-For，期望 200，实际 %d", rr.Code)
	}
}

// TestExtractClientIP 测试客户端 IP 提取逻辑
func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name      string
		remoteAddr string
		xRealIP   string
		xForwFor  string
		wantIP    string
	}{
		{
			name:       "仅 RemoteAddr（含端口）",
			remoteAddr: "192.168.1.1:12345",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "X-Real-IP 优先",
			remoteAddr: "192.168.1.1:12345",
			xRealIP:    "10.0.0.1",
			wantIP:     "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For 取第一个",
			remoteAddr: "192.168.1.1:12345",
			xForwFor:   "10.0.0.2, 172.16.0.1",
			wantIP:     "10.0.0.2",
		},
		{
			name:       "X-Real-IP 优先于 X-Forwarded-For",
			remoteAddr: "192.168.1.1:12345",
			xRealIP:    "10.0.0.1",
			xForwFor:   "10.0.0.2, 172.16.0.1",
			wantIP:     "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if tt.xForwFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwFor)
			}

			got := extractClientIP(req)
			if got != tt.wantIP {
				t.Errorf("期望 IP %s，实际 %s", tt.wantIP, got)
			}
		})
	}
}

// TestIPWhitelistMixedConfig 测试精确 IP 和 CIDR 混合配置
func TestIPWhitelistMixedConfig(t *testing.T) {
	cfg := &IPWhitelistConfig{
		Enabled:   true,
		AllowList: []string{"192.168.1.100", "10.0.0.0/8"},
	}

	handler := IPWhitelistMiddleware(cfg)(okHandler())

	tests := []struct {
		name       string
		ip         string
		wantStatus int
	}{
		{"精确匹配 IP", "192.168.1.100:8080", http.StatusOK},
		{"CIDR 匹配", "10.1.2.3:8080", http.StatusOK},
		{"都不匹配", "172.16.0.1:8080", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.ip
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("IP %s: 期望 %d，实际 %d", tt.ip, tt.wantStatus, rr.Code)
			}
		})
	}
}
