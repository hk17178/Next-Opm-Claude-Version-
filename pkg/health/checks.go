// checks.go 提供常用的健康检查函数实现，包括 TCP 连通性检查和数据库 Ping 检查。

package health

import (
	"context"
	"fmt"
	"net"
	"time"
)

// PingCheck 返回一个通过 TCP 连接验证 host:port 连通性的检查函数。
// 适用于数据库、Redis、Kafka Broker 等依赖的健康检查。
func PingCheck(addr string, timeout time.Duration) Check {
	return func(ctx context.Context) error {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return fmt.Errorf("cannot reach %s: %w", addr, err)
		}
		conn.Close()
		return nil
	}
}

// Pinger 是支持 Ping 方法的接口，任何实现了 Ping(context.Context) error 的类型
// 均可用于数据库健康检查（例如 *pgxpool.Pool）。
type Pinger interface {
	Ping(ctx context.Context) error
}

// DatabaseCheck 返回一个通过 Ping 方法验证数据库连接的检查函数。
func DatabaseCheck(db Pinger) Check {
	return func(ctx context.Context) error {
		if err := db.Ping(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
		return nil
	}
}
