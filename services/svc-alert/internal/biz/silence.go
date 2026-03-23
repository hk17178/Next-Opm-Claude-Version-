// Package biz 实现告警服务的核心业务逻辑，包含告警沉默规则的用例层。
package biz

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SilenceUseCase 封装告警沉默规则（Silence）的业务操作，负责校验、生命周期管理
// 以及与 SilenceRepository 的交互，将沉默规则持久化到 PostgreSQL。
//
// 沉默规则（Silence）的作用：在指定时间窗口内，符合标签匹配条件的告警将被静默，
// 不会发送通知，适用于计划内维护、上线窗口期等场景。
type SilenceUseCase struct {
	// repo 是沉默规则的持久化仓储，实现了 SilenceRepository 接口
	repo SilenceRepository
	// log 是结构化日志记录器，用于记录操作结果和错误
	log *zap.SugaredLogger
}

// NewSilenceUseCase 创建沉默规则用例实例。
//
// 参数：
//   - repo：沉默规则仓储接口（通常为 data.SilenceRepo）。
//   - log：结构化日志记录器（zap.SugaredLogger）。
//
// 返回：已初始化的 SilenceUseCase 实例。
func NewSilenceUseCase(repo SilenceRepository, log *zap.SugaredLogger) *SilenceUseCase {
	return &SilenceUseCase{
		repo: repo,
		log:  log,
	}
}

// CreateSilence 创建并持久化一条告警沉默规则到 PostgreSQL。
//
// 业务逻辑：
//  1. 校验必填字段：Matchers 不能为空，EndsAt 不能为空，且结束时间必须晚于开始时间。
//  2. 自动生成 UUID 作为沉默规则 ID（若调用方未指定）。
//  3. 若 StartsAt 为零值，则自动设置为当前时间（立即生效）。
//  4. 将规则写入数据库并记录操作日志。
//
// 参数：
//   - s：待创建的沉默规则，至少需要包含 Matchers 和 EndsAt。
//
// 返回：创建失败时返回具体错误，成功时返回 nil。
func (uc *SilenceUseCase) CreateSilence(s *Silence) error {
	// 校验：至少需要一个匹配器，否则会静默所有告警，风险过高
	if len(s.Matchers) == 0 {
		return errors.New("at least one matcher is required")
	}

	// 校验：结束时间是必填字段，没有结束时间的沉默规则无意义
	if s.EndsAt.IsZero() {
		return errors.New("ends_at is required")
	}

	// 设置默认开始时间：若未指定，则立即生效
	if s.StartsAt.IsZero() {
		s.StartsAt = time.Now()
	}

	// 业务约束：结束时间必须晚于开始时间
	if !s.EndsAt.After(s.StartsAt) {
		return errors.New("ends_at must be after starts_at")
	}

	// 自动生成唯一标识符
	if s.ID == "" {
		s.ID = uuid.New().String()
	}

	// 持久化到 PostgreSQL，写入 silences 表
	if err := uc.repo.Create(s); err != nil {
		return fmt.Errorf("create silence: %w", err)
	}

	uc.log.Infow("silence rule created",
		"id", s.ID,
		"matchers_count", len(s.Matchers),
		"starts_at", s.StartsAt,
		"ends_at", s.EndsAt,
		"created_by", s.CreatedBy,
	)

	return nil
}

// ListSilences 查询所有沉默规则，包括已过期的历史记录。
// 返回结果由 SilenceRepository 实现按 created_at 倒序排列。
//
// 返回：
//   - []*Silence：沉默规则列表（若无数据则返回空切片）。
//   - error：查询失败时返回数据库错误。
func (uc *SilenceUseCase) ListSilences() ([]*Silence, error) {
	silences, err := uc.repo.List()
	if err != nil {
		return nil, fmt.Errorf("list silences: %w", err)
	}

	// 保证返回空切片而非 nil，避免 JSON 序列化时出现 null
	if silences == nil {
		silences = []*Silence{}
	}

	return silences, nil
}

// DeleteSilence 按 ID 删除沉默规则（提前解除静默）。
// 仅当 SilenceRepository 支持 Delete 方法时可用；若接口未定义 Delete，
// 可通过将 ends_at 设置为过去时间变相"删除"（需扩展接口）。
//
// 注意：当前 SilenceRepository 接口未定义 Delete，此方法为预留扩展点，
// 若仓储层实现了 Delete 则可调用；否则返回 "not supported" 错误。
//
// 参数：
//   - id：要删除的沉默规则 UUID。
//
// 返回：删除成功返回 nil，不支持或失败返回错误。
func (uc *SilenceUseCase) DeleteSilence(id string) error {
	if id == "" {
		return errors.New("silence id is required")
	}

	// SilenceRepository 接口目前包含 Create/List/GetActive 三个方法。
	// Delete 是扩展功能：通过类型断言检测仓储是否实现了可选的 SilenceDeletable 接口。
	type SilenceDeletable interface {
		Delete(id string) error
	}

	if deletable, ok := uc.repo.(SilenceDeletable); ok {
		// 仓储实现了 Delete 方法，直接调用
		if err := deletable.Delete(id); err != nil {
			return fmt.Errorf("delete silence %s: %w", id, err)
		}
		uc.log.Infow("silence rule deleted", "id", id)
		return nil
	}

	// 仓储未实现 Delete，返回明确错误提示调用方
	return fmt.Errorf("silence delete not supported by current repository implementation (id: %s)", id)
}

// GetActiveSilences 查询当前时间内有效的、匹配给定标签集的沉默规则。
// 用于告警引擎判断某条告警是否应被静默处理。
//
// 参数：
//   - labels：告警的标签键值对，用于与沉默规则的 matchers 进行匹配。
//
// 返回：
//   - []*Silence：当前有效且匹配的沉默规则列表。
//   - error：查询失败时返回错误。
func (uc *SilenceUseCase) GetActiveSilences(labels map[string]string) ([]*Silence, error) {
	silences, err := uc.repo.GetActive(labels)
	if err != nil {
		return nil, fmt.Errorf("get active silences: %w", err)
	}

	if silences == nil {
		silences = []*Silence{}
	}

	return silences, nil
}

// IsSilenced 判断一个携带给定标签的告警是否被某条沉默规则覆盖。
// 这是告警引擎调用的便捷方法，返回布尔值和匹配到的第一条沉默规则（用于日志记录）。
//
// 参数：
//   - labels：告警的标签键值对。
//
// 返回：
//   - bool：true 表示告警被静默，不应发送通知。
//   - *Silence：触发静默的第一条规则，若未被静默则为 nil。
//   - error：查询失败时返回错误。
func (uc *SilenceUseCase) IsSilenced(labels map[string]string) (bool, *Silence, error) {
	actives, err := uc.GetActiveSilences(labels)
	if err != nil {
		return false, nil, err
	}

	if len(actives) > 0 {
		// 存在匹配的沉默规则，返回第一条规则用于日志记录
		return true, actives[0], nil
	}

	return false, nil, nil
}
