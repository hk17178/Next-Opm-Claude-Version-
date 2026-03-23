package biz

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RuleUseCase 封装告警规则的管理操作，包括 CRUD 和业务校验逻辑。
type RuleUseCase struct {
	repo RuleRepository
	log  *zap.SugaredLogger
}

// NewRuleUseCase 创建规则用例实例，注入规则仓储和日志器。
func NewRuleUseCase(repo RuleRepository, log *zap.SugaredLogger) *RuleUseCase {
	return &RuleUseCase{repo: repo, log: log}
}

// CreateRule 创建告警规则，执行必填字段校验、自动生成 UUID 和时间戳。
// 冷却时间默认 5 分钟，Layer 必须在 0-5 之间。
func (uc *RuleUseCase) CreateRule(rule *AlertRule) error {
	if rule.Name == "" {
		return errors.New("rule name is required")
	}
	if rule.Condition == nil {
		return errors.New("rule condition is required")
	}
	if rule.Severity == "" {
		return errors.New("rule severity is required")
	}
	if rule.Layer < 0 || rule.Layer > 5 {
		return fmt.Errorf("rule layer must be between 0 and 5, got %d", rule.Layer)
	}

	now := time.Now()
	rule.RuleID = uuid.New().String()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.Enabled = true

	if rule.CooldownMinutes == 0 {
		rule.CooldownMinutes = 5
	}

	if err := uc.repo.Create(rule); err != nil {
		return err
	}

	uc.log.Infow("rule created", "id", rule.RuleID, "name", rule.Name,
		"layer", rule.Layer, "type", rule.RuleType, "ironclad", rule.Ironclad)
	return nil
}

// UpdateRule 更新告警规则，保留原始创建时间并刷新更新时间。
func (uc *RuleUseCase) UpdateRule(rule *AlertRule) error {
	existing, err := uc.repo.GetByID(rule.RuleID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("rule not found")
	}

	rule.UpdatedAt = time.Now()
	rule.CreatedAt = existing.CreatedAt

	if err := uc.repo.Update(rule); err != nil {
		return err
	}

	uc.log.Infow("rule updated", "id", rule.RuleID, "name", rule.Name)
	return nil
}

// DeleteRule 按 ID 删除告警规则。
func (uc *RuleUseCase) DeleteRule(id string) error {
	if err := uc.repo.Delete(id); err != nil {
		return err
	}
	uc.log.Infow("rule deleted", "id", id)
	return nil
}

// GetRule 按 ID 查询单条告警规则。
func (uc *RuleUseCase) GetRule(id string) (*AlertRule, error) {
	return uc.repo.GetByID(id)
}

// EnableRule 启用告警规则，状态变更立即生效（引擎每次评估都从数据库加载最新规则）。
func (uc *RuleUseCase) EnableRule(id string) error {
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("rule not found")
	}

	existing.Enabled = true
	existing.UpdatedAt = time.Now()
	if err := uc.repo.Update(existing); err != nil {
		return err
	}
	uc.log.Infow("rule enabled", "id", id, "name", existing.Name)
	return nil
}

// DisableRule 禁用告警规则，状态变更立即生效（引擎每次评估都从数据库加载最新规则）。
func (uc *RuleUseCase) DisableRule(id string) error {
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("rule not found")
	}

	existing.Enabled = false
	existing.UpdatedAt = time.Now()
	if err := uc.repo.Update(existing); err != nil {
		return err
	}
	uc.log.Infow("rule disabled", "id", id, "name", existing.Name)
	return nil
}

// ListRules 分页查询告警规则列表，支持按启用状态过滤，pageSize 上限 100。
func (uc *RuleUseCase) ListRules(enabled *bool, pageSize int, pageToken string) ([]*AlertRule, string, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.List(enabled, pageSize, pageToken)
}
