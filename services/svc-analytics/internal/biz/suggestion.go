// suggestion.go 实现用户改进建议管理业务逻辑（模块 16，FR-16-001~006）。
// 支持建议提交、AI 分类/关键词提取/情感分析/去重、状态流转和统计分析。

package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SuggestionStatus 定义建议状态枚举类型。
type SuggestionStatus string

// 建议状态常量，覆盖完整生命周期。
const (
	SuggestionStatusPending    SuggestionStatus = "pending"     // 待审核
	SuggestionStatusAccepted   SuggestionStatus = "accepted"    // 已采纳
	SuggestionStatusRejected   SuggestionStatus = "rejected"    // 已拒绝
	SuggestionStatusInProgress SuggestionStatus = "in_progress" // 实施中
	SuggestionStatusLaunched   SuggestionStatus = "launched"    // 已上线
)

// validSuggestionStatuses 定义所有合法的建议状态值，用于参数校验。
var validSuggestionStatuses = map[SuggestionStatus]bool{
	SuggestionStatusPending:    true,
	SuggestionStatusAccepted:   true,
	SuggestionStatusRejected:   true,
	SuggestionStatusInProgress: true,
	SuggestionStatusLaunched:   true,
}

// Suggestion 用户改进建议实体（FR-16-001）。
// 包含 AI 自动分类、关键词提取、情感识别和相似建议去重等字段。
type Suggestion struct {
	ID          string           `json:"id" db:"id"`                    // UUID 主键
	Title       string           `json:"title" db:"title"`              // 建议标题
	Description string           `json:"description" db:"description"`  // 详细描述
	SubmittedBy string           `json:"submitted_by" db:"submitted_by"` // 提交人 ID
	Status      SuggestionStatus `json:"status" db:"status"`            // 当前状态
	Category    string           `json:"category" db:"category"`        // AI 分类：功能/体验/性能/安全/流程
	Keywords    []string         `json:"keywords" db:"keywords"`        // AI 提取的关键词
	Sentiment   string           `json:"sentiment" db:"sentiment"`      // AI 情感识别：positive/neutral/negative
	SimilarIDs  []string         `json:"similar_ids" db:"similar_ids"`  // 相似建议 ID（AI 去重）
	AdminNote   string           `json:"admin_note" db:"admin_note"`    // 管理员备注
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`    // 创建时间
	UpdatedAt   time.Time        `json:"updated_at" db:"updated_at"`    // 更新时间
}

// SuggestionStats 建议统计数据（FR-16-006）。
// 包含各状态计数、采纳率和平均处理周期。
type SuggestionStats struct {
	Total              int     `json:"total"`                // 总建议数
	Pending            int     `json:"pending"`              // 待审核数
	Accepted           int     `json:"accepted"`             // 已采纳数
	Rejected           int     `json:"rejected"`             // 已拒绝数
	InProgress         int     `json:"in_progress"`          // 实施中数
	Launched           int     `json:"launched"`             // 已上线数
	AcceptanceRate     float64 `json:"acceptance_rate"`      // 采纳率（已采纳+实施中+已上线 / 总数）
	AvgProcessingDays  float64 `json:"avg_processing_days"`  // 平均处理周期（天）
}

// SuggestionRepo 定义建议数据访问接口（PostgreSQL 存储）。
type SuggestionRepo interface {
	// Create 创建新建议
	Create(ctx context.Context, s *Suggestion) error
	// Get 根据 ID 获取单条建议
	Get(ctx context.Context, id string) (*Suggestion, error)
	// List 分页查询建议列表，支持按状态筛选
	List(ctx context.Context, status string, page, pageSize int) ([]*Suggestion, int, error)
	// UpdateStatus 更新建议状态和管理员备注
	UpdateStatus(ctx context.Context, id string, status SuggestionStatus, adminNote string) error
	// GetStats 获取建议统计数据
	GetStats(ctx context.Context) (*SuggestionStats, error)
}

// SuggestionAIService 定义建议 AI 分析接口（分类、关键词、情感、去重）。
type SuggestionAIService interface {
	// Classify 对建议进行 AI 分类，返回分类标签
	Classify(ctx context.Context, title, description string) (category string, err error)
	// ExtractKeywords 提取建议中的关键词
	ExtractKeywords(ctx context.Context, title, description string) ([]string, error)
	// AnalyzeSentiment 分析建议的情感倾向
	AnalyzeSentiment(ctx context.Context, title, description string) (sentiment string, err error)
	// FindSimilar 查找与当前建议相似的已有建议 ID
	FindSimilar(ctx context.Context, title, description string) ([]string, error)
}

// SuggestionUsecase 用户建议业务逻辑用例。
// 整合数据存储和 AI 分析能力，实现建议的完整生命周期管理。
type SuggestionUsecase struct {
	repo   SuggestionRepo       // 数据访问层
	ai     SuggestionAIService  // AI 分析服务（可选）
	logger *zap.Logger          // 日志记录器
}

// NewSuggestionUsecase 创建建议业务用例实例。
// ai 参数可为 nil，此时跳过 AI 分析功能。
func NewSuggestionUsecase(repo SuggestionRepo, ai SuggestionAIService, logger *zap.Logger) *SuggestionUsecase {
	return &SuggestionUsecase{
		repo:   repo,
		ai:     ai,
		logger: logger,
	}
}

// Submit 提交新的用户建议（FR-16-001）。
// 自动触发 AI 分类、关键词提取、情感分析和相似建议去重。
// AI 分析失败不会阻断建议提交，仅记录警告日志。
func (uc *SuggestionUsecase) Submit(ctx context.Context, title, description, userID string) (*Suggestion, error) {
	// 参数校验
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// 构建建议实体
	s := &Suggestion{
		Title:       title,
		Description: description,
		SubmittedBy: userID,
		Status:      SuggestionStatusPending,
	}

	// AI 分析（可选，失败不阻断提交）
	if uc.ai != nil {
		// FR-16-002: AI 自动分类
		if category, err := uc.ai.Classify(ctx, title, description); err != nil {
			uc.logger.Warn("AI 分类失败", zap.Error(err))
		} else {
			s.Category = category
		}

		// FR-16-003: AI 关键词提取
		if keywords, err := uc.ai.ExtractKeywords(ctx, title, description); err != nil {
			uc.logger.Warn("AI 关键词提取失败", zap.Error(err))
		} else {
			s.Keywords = keywords
		}

		// FR-16-004: AI 情感分析
		if sentiment, err := uc.ai.AnalyzeSentiment(ctx, title, description); err != nil {
			uc.logger.Warn("AI 情感分析失败", zap.Error(err))
		} else {
			s.Sentiment = sentiment
		}

		// FR-16-005: AI 相似建议去重
		if similarIDs, err := uc.ai.FindSimilar(ctx, title, description); err != nil {
			uc.logger.Warn("AI 相似建议查找失败", zap.Error(err))
		} else {
			s.SimilarIDs = similarIDs
		}
	}

	// 持久化到数据库
	if err := uc.repo.Create(ctx, s); err != nil {
		return nil, fmt.Errorf("create suggestion: %w", err)
	}

	return s, nil
}

// List 分页查询建议列表（FR-16-001），支持按状态筛选。
// status 为空字符串时返回所有状态的建议。
func (uc *SuggestionUsecase) List(ctx context.Context, status string, page, pageSize int) ([]*Suggestion, int, error) {
	// 校验状态参数（如果提供了的话）
	if status != "" {
		if !validSuggestionStatuses[SuggestionStatus(status)] {
			return nil, 0, fmt.Errorf("invalid status: %s", status)
		}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return uc.repo.List(ctx, status, page, pageSize)
}

// UpdateStatus 更新建议状态（FR-16-001）。
// 支持管理员添加备注说明状态变更原因。
func (uc *SuggestionUsecase) UpdateStatus(ctx context.Context, id, status, adminNote string) error {
	if id == "" {
		return fmt.Errorf("suggestion id is required")
	}
	st := SuggestionStatus(status)
	if !validSuggestionStatuses[st] {
		return fmt.Errorf("invalid status: %s", status)
	}
	return uc.repo.UpdateStatus(ctx, id, st, adminNote)
}

// GetStats 获取建议统计数据（FR-16-006）。
// 返回各状态计数、采纳率和平均处理周期。
func (uc *SuggestionUsecase) GetStats(ctx context.Context) (*SuggestionStats, error) {
	return uc.repo.GetStats(ctx)
}
