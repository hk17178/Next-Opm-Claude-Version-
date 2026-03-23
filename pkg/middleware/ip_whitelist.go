// ip_whitelist.go 实现 IP 白名单中间件，仅允许白名单内的客户端 IP 访问受保护资源。
// 支持 IPv4/IPv6 精确匹配、CIDR 网段匹配和带过期时间的临时白名单。

package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TempEntry 临时白名单条目，包含 IP 或 CIDR 及其过期时间
type TempEntry struct {
	CIDR      string    // 允许的 IP 或 CIDR 地址
	ExpiresAt time.Time // 过期时间，超过此时间自动失效
}

// IPWhitelistConfig IP 白名单配置
type IPWhitelistConfig struct {
	Enabled   bool        // 是否启用白名单
	AllowList []string    // 允许的 IP 或 CIDR，如 "192.168.1.0/24"
	TempAllow []TempEntry // 临时白名单（含过期时间）
}

// ipWhitelist 内部白名单状态，解析后的网段列表和临时条目
type ipWhitelist struct {
	mu          sync.RWMutex
	enabled     bool
	exactIPs    map[string]struct{} // 精确匹配的 IP 地址集合
	cidrNets    []*net.IPNet        // CIDR 网段列表
	tempEntries []tempEntryParsed   // 已解析的临时白名单条目
}

// tempEntryParsed 已解析的临时白名单条目
type tempEntryParsed struct {
	ip        net.IP     // 精确 IP（若非 CIDR 则设置）
	network   *net.IPNet // CIDR 网段（若是 CIDR 则设置）
	expiresAt time.Time  // 过期时间
}

// parseConfig 将配置解析为内部白名单结构
func parseConfig(cfg *IPWhitelistConfig) *ipWhitelist {
	wl := &ipWhitelist{
		enabled:  cfg.Enabled,
		exactIPs: make(map[string]struct{}),
	}

	// 解析静态白名单列表
	for _, entry := range cfg.AllowList {
		if strings.Contains(entry, "/") {
			// CIDR 网段格式
			_, network, err := net.ParseCIDR(entry)
			if err == nil {
				wl.cidrNets = append(wl.cidrNets, network)
			}
		} else {
			// 精确 IP 地址
			ip := net.ParseIP(entry)
			if ip != nil {
				wl.exactIPs[ip.String()] = struct{}{}
			}
		}
	}

	// 解析临时白名单条目
	for _, temp := range cfg.TempAllow {
		parsed := tempEntryParsed{
			expiresAt: temp.ExpiresAt,
		}
		if strings.Contains(temp.CIDR, "/") {
			_, network, err := net.ParseCIDR(temp.CIDR)
			if err == nil {
				parsed.network = network
			}
		} else {
			ip := net.ParseIP(temp.CIDR)
			if ip != nil {
				parsed.ip = ip
			}
		}
		wl.tempEntries = append(wl.tempEntries, parsed)
	}

	return wl
}

// isAllowed 检查给定 IP 是否在白名单中
func (wl *ipWhitelist) isAllowed(ipStr string) bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查精确匹配
	if _, ok := wl.exactIPs[ip.String()]; ok {
		return true
	}

	// 检查 CIDR 网段匹配
	for _, network := range wl.cidrNets {
		if network.Contains(ip) {
			return true
		}
	}

	// 检查临时白名单（需排除已过期条目）
	now := time.Now()
	for _, temp := range wl.tempEntries {
		// 跳过已过期的临时条目
		if now.After(temp.expiresAt) {
			continue
		}
		if temp.network != nil && temp.network.Contains(ip) {
			return true
		}
		if temp.ip != nil && temp.ip.Equal(ip) {
			return true
		}
	}

	return false
}

// extractClientIP 从请求中提取真实客户端 IP 地址。
// 优先从 X-Real-IP 获取，其次从 X-Forwarded-For 的第一个值获取，最后使用 RemoteAddr。
func extractClientIP(r *http.Request) string {
	// 优先使用 X-Real-IP 头
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ip := strings.TrimSpace(realIP)
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	// 其次使用 X-Forwarded-For 头（取第一个 IP）
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For 格式：client, proxy1, proxy2
		parts := strings.SplitN(forwarded, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	// 最后使用 RemoteAddr（可能包含端口号）
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr 没有端口号的情况
		return r.RemoteAddr
	}
	return host
}

// IPWhitelistMiddleware 返回 IP 白名单 HTTP 中间件。
// 白名单未启用时直接放行，启用后仅允许白名单内的 IP 通过，其余返回 403 Forbidden。
func IPWhitelistMiddleware(cfg *IPWhitelistConfig) func(http.Handler) http.Handler {
	wl := parseConfig(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 白名单未启用时直接放行
			if !wl.enabled {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := extractClientIP(r)

			if !wl.isAllowed(clientIP) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"code":"security.ip.blocked","message":"IP address not in whitelist"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
