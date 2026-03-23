package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// OnCallPerson 封装当前值班人员信息，在 HTTP 响应和日志中使用。
type OnCallPerson struct {
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	ScheduleID string `json:"schedule_id"`
	// IsOverride 标识该值班人员是通过替班记录（override）确定的，而非常规轮转。
	IsOverride bool `json:"is_override"`
}

// OncallUsecase 封装值班排班的业务逻辑，提供通过排班 ID 查询当前值班人员的能力。
type OncallUsecase struct {
	// scheduleGetByID 是一个函数类型依赖，通过闭包方式注入，避免引入新的接口类型。
	// 在生产环境中由 ScheduleRepo.GetByID 提供实现；测试环境可直接注入 mock。
	scheduleGetByID func(ctx context.Context, id string) (*OncallSchedule, error)
}

// NewOncallUsecase 创建一个新的 OncallUsecase，注入排班数据获取函数。
//
// 参数：
//   - getByID: 根据排班 ID 查询 OncallSchedule 的函数，通常由 data.ScheduleRepo.GetByID 提供。
func NewOncallUsecase(getByID func(ctx context.Context, id string) (*OncallSchedule, error)) *OncallUsecase {
	return &OncallUsecase{scheduleGetByID: getByID}
}

// GetCurrentOnCallPersonByScheduleID 根据排班计划 ID 查询当前时刻的值班人员。
//
// 业务逻辑：
//  1. 通过排班 ID 从数据库加载排班配置（rotation JSON）。
//  2. 优先检查替班记录（on_call_overrides），如果当前时间落在某个替班区间内，返回替班人员。
//  3. 若无替班，根据轮转类型（daily/weekly/custom）和起始日期，通过取模算法计算当前轮到的成员索引。
//
// 参数：
//   - ctx: 请求上下文。
//   - scheduleID: oncall_schedules 表的主键 UUID。
//
// 返回值：
//   - *OnCallPerson: 当前值班人员信息，包含 UserID、Name、ScheduleID 和是否为替班标记。
//   - error: 排班不存在时返回 NotFound 错误；成员列表为空时返回业务错误。
func (uc *OncallUsecase) GetCurrentOnCallPersonByScheduleID(ctx context.Context, scheduleID string) (*OnCallPerson, error) {
	// 从数据库加载排班配置
	schedule, err := uc.scheduleGetByID(ctx, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("load schedule %s: %w", scheduleID, err)
	}
	if schedule == nil {
		return nil, fmt.Errorf("schedule %s not found", scheduleID)
	}
	if !schedule.Enabled {
		return nil, fmt.Errorf("schedule %s is disabled", scheduleID)
	}

	// 解析排班轮转配置和替班记录
	rotation, overrides := parseScheduleConfig(schedule)
	if rotation == nil || len(rotation.Members) == 0 {
		return nil, fmt.Errorf("schedule %s has no members configured", scheduleID)
	}

	now := time.Now()

	// 优先检查替班记录：如果当前时间在替班区间内，直接返回替班人员
	if member := checkOverrides(overrides, now); member != nil {
		return &OnCallPerson{
			UserID:     member.UserID,
			Name:       member.Name,
			ScheduleID: scheduleID,
			IsOverride: true, // 标记为替班，便于调用方区分常规值班和临时替班
		}, nil
	}

	// 通过取模算法计算当前轮到的成员
	member := calculateRotation(rotation, now)
	if member == nil {
		return nil, fmt.Errorf("failed to calculate on-call member for schedule %s", scheduleID)
	}

	return &OnCallPerson{
		UserID:     member.UserID,
		Name:       member.Name,
		ScheduleID: scheduleID,
		IsOverride: false,
	}, nil
}

// GetCurrentOnCallPerson 根据排班计划的轮转配置，计算指定时间点的当前值班人员。
// 支持 daily（每日轮转）、weekly（每周轮转）和 custom（自定义天数轮转）三种模式。
// 同时检查替班（override）记录，如果当前时间有替班则返回替班人员。
func GetCurrentOnCallPerson(schedule *OncallSchedule, at time.Time) *OnCallMember {
	rotation, overrides := parseScheduleConfig(schedule)
	if rotation == nil || len(rotation.Members) == 0 {
		return nil
	}

	// 先检查替班记录
	if member := checkOverrides(overrides, at); member != nil {
		return member
	}

	// 计算轮转索引
	return calculateRotation(rotation, at)
}

// parseScheduleConfig 从 OncallSchedule 的 Rotation 和 Overrides 字段解析出结构化配置。
func parseScheduleConfig(schedule *OncallSchedule) (*RotationConfig, []OverrideEntry) {
	var rotation RotationConfig
	rotationBytes, err := json.Marshal(schedule.Rotation)
	if err != nil {
		return nil, nil
	}
	if err := json.Unmarshal(rotationBytes, &rotation); err != nil {
		return nil, nil
	}

	var overrides []OverrideEntry
	if schedule.Overrides != nil {
		overridesBytes, _ := json.Marshal(schedule.Overrides)
		json.Unmarshal(overridesBytes, &overrides)
	}

	return &rotation, overrides
}

// checkOverrides 检查替班记录，如果 at 时间落在某个替班区间内则返回替班人员。
func checkOverrides(overrides []OverrideEntry, at time.Time) *OnCallMember {
	atDate := at.Format("2006-01-02")
	for _, o := range overrides {
		if o.StartDate <= atDate && atDate <= o.EndDate {
			return &OnCallMember{
				UserID: o.UserID,
				Name:   o.Name,
			}
		}
	}
	return nil
}

// calculateRotation 根据轮转类型和起始日期计算当前值班人员的索引。
func calculateRotation(config *RotationConfig, at time.Time) *OnCallMember {
	if len(config.Members) == 0 {
		return nil
	}

	startDate, err := time.Parse("2006-01-02", config.StartDate)
	if err != nil {
		// 如果起始日期解析失败，默认返回第一个成员
		return &config.Members[0]
	}

	daysSinceStart := int(at.Sub(startDate).Hours() / 24)
	if daysSinceStart < 0 {
		// 排班尚未开始，返回第一个成员
		return &config.Members[0]
	}

	memberCount := len(config.Members)
	var idx int

	switch config.Type {
	case RotationDaily:
		idx = daysSinceStart % memberCount
	case RotationWeekly:
		weeksSinceStart := daysSinceStart / 7
		idx = weeksSinceStart % memberCount
	case RotationCustom:
		shiftDays := config.ShiftDays
		if shiftDays <= 0 {
			shiftDays = 1
		}
		shiftsSinceStart := daysSinceStart / shiftDays
		idx = shiftsSinceStart % memberCount
	default:
		idx = daysSinceStart % memberCount
	}

	return &config.Members[idx]
}
