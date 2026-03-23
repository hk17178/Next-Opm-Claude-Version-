// Package biz 定义日志服务的核心业务模型与领域逻辑，包括日志条目、采集源、解析规则、脱敏规则、保留策略等。
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LogRepository 定义日志配置的关系型存储持久化接口，涵盖采集源、解析规则、脱敏规则、保留策略和日志流的 CRUD 操作。
type LogRepository interface {
	// LogSource CRUD（日志采集源增删改查）
	CreateLogSource(ctx context.Context, src *LogSource) error
	GetLogSource(ctx context.Context, id string) (*LogSource, error)
	ListLogSources(ctx context.Context) ([]*LogSource, error)
	UpdateLogSource(ctx context.Context, src *LogSource) error
	DeleteLogSource(ctx context.Context, id string) error

	// ParseRule CRUD（解析规则增删改查）
	CreateParseRule(ctx context.Context, rule *ParseRule) error
	GetParseRule(ctx context.Context, id string) (*ParseRule, error)
	ListParseRules(ctx context.Context) ([]*ParseRule, error)
	UpdateParseRule(ctx context.Context, rule *ParseRule) error
	DeleteParseRule(ctx context.Context, id string) error

	// MaskingRule CRUD（脱敏规则增删改查）
	CreateMaskingRule(ctx context.Context, rule *MaskingRule) error
	GetMaskingRule(ctx context.Context, id string) (*MaskingRule, error)
	ListMaskingRules(ctx context.Context) ([]*MaskingRule, error)
	UpdateMaskingRule(ctx context.Context, rule *MaskingRule) error
	DeleteMaskingRule(ctx context.Context, id string) error

	// RetentionPolicy CRUD（保留策略增删改查）
	CreateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error
	GetRetentionPolicy(ctx context.Context, id string) (*RetentionPolicy, error)
	ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error)
	UpdateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error
	DeleteRetentionPolicy(ctx context.Context, id string) error

	// Stream CRUD（日志流增删改查）
	CreateStream(ctx context.Context, s *Stream) error
	GetStream(ctx context.Context, id string) (*Stream, error)
	ListStreams(ctx context.Context, pageSize int, pageToken string) ([]*Stream, string, error)
	DeleteStream(ctx context.Context, id string) error
}

// ESRepository 定义 Elasticsearch 的日志写入和查询接口。
type ESRepository interface {
	BulkIndex(ctx context.Context, entries []LogEntry) error
	Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error)
	Aggregate(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error)
	ListIndices(ctx context.Context, pattern string) ([]IndexInfo, error)
	DeleteIndex(ctx context.Context, indexName string) error
}

// EventPublisher 定义向 Kafka 发布领域事件的接口。
type EventPublisher interface {
	Publish(ctx context.Context, topic string, event CloudEvent) error
}

// IngestConfig 保存日志摄入管道的配置参数。
type IngestConfig struct {
	MaxBatchSize    int
	FlushInterval   time.Duration
	SensitiveFields []string
}

// IngestService 处理日志接收、解析、富化和脱敏，内部采用缓冲区批量写入 ES。
type IngestService struct {
	logRepo   LogRepository
	esRepo    ESRepository
	publisher EventPublisher
	logger    *zap.Logger
	config    IngestConfig

	mu     sync.Mutex
	buffer []LogEntry
	timer  *time.Timer
}

// NewIngestService 创建一个带缓冲写入的日志摄入服务实例，并启动定时刷新定时器。
func NewIngestService(logRepo LogRepository, esRepo ESRepository, publisher EventPublisher, logger *zap.Logger, cfg IngestConfig) *IngestService {
	s := &IngestService{
		logRepo:   logRepo,
		esRepo:    esRepo,
		publisher: publisher,
		logger:    logger,
		config:    cfg,
		buffer:    make([]LogEntry, 0, cfg.MaxBatchSize),
	}
	s.timer = time.AfterFunc(cfg.FlushInterval, func() {
		if err := s.Flush(context.Background()); err != nil {
			logger.Error("periodic flush failed", zap.Error(err))
		}
	})
	return s
}

// IngestHTTP 通过 HTTP 接收一批日志条目，逐条校验、生成 ID、脱敏后加入缓冲区。
// 请求/响应与 OpenAPI LogIngestRequest/LogIngestResponse 对齐。
func (s *IngestService) IngestHTTP(ctx context.Context, req LogIngestRequest) (*LogIngestResponse, error) {
	maskingRules, _ := s.logRepo.ListMaskingRules(ctx)

	resp := &LogIngestResponse{}
	var accepted []LogEntry

	for i, entry := range req.Entries {
		if err := validateEntry(&entry); err != nil {
			resp.Rejected++
			resp.Errors = append(resp.Errors, IngestError{Index: i, Reason: err.Error()})
			continue
		}

		entry.ID = generateID()
		entry.CreatedAt = time.Now().UTC()
		if entry.Timestamp.IsZero() {
			entry.Timestamp = entry.CreatedAt
		}

		s.applyMasking(&entry, maskingRules)
		s.maskSensitiveFields(&entry)

		accepted = append(accepted, entry)
		resp.Accepted++
	}

	if len(accepted) > 0 {
		s.addToBuffer(accepted)
	}

	return resp, nil
}

// IngestKafka 处理单条 Kafka 消息，将其反序列化后送入摄入管道。
// 支持批量和单条两种消息格式。
//
// 修复说明：json.Unmarshal 对于不含 "entries" 字段的单条日志 JSON（如 {"message":"..."}）
// 会成功解析为空 LogIngestRequest（Entries 为 nil），导致单条消息被静默丢弃。
// 修复方案：解析成功后额外检查 Entries 是否为空，若为空则回退为单条 LogEntry 解析。
func (s *IngestService) IngestKafka(ctx context.Context, key, value []byte) error {
	var req LogIngestRequest
	if err := json.Unmarshal(value, &req); err != nil || len(req.Entries) == 0 {
		// json.Unmarshal 失败，或成功但 Entries 为空（单条日志格式），
		// 回退为单条 LogEntry 解析。
		var entry LogEntry
		if err2 := json.Unmarshal(value, &entry); err2 != nil {
			if err != nil {
				return fmt.Errorf("unmarshal kafka message: %w", err)
			}
			return fmt.Errorf("unmarshal kafka message as single entry: %w", err2)
		}
		req.Entries = []LogEntry{entry}
	}

	_, err := s.IngestHTTP(ctx, req)
	return err
}

// Flush 将缓冲区中所有日志条目批量写入 Elasticsearch，并发布批次摄入事件。
// 写入失败时将条目重新放回缓冲区，超过上限则丢弃最旧的条目。
func (s *IngestService) Flush(ctx context.Context) error {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		s.resetTimer()
		return nil
	}
	batch := s.buffer
	s.buffer = make([]LogEntry, 0, s.config.MaxBatchSize)
	s.mu.Unlock()

	s.logger.Info("flushing log batch", zap.Int("count", len(batch)))
	err := s.esRepo.BulkIndex(ctx, batch)
	if err != nil {
		s.logger.Error("bulk index failed", zap.Error(err), zap.Int("count", len(batch)))
		s.mu.Lock()
		s.buffer = append(batch, s.buffer...)
		if len(s.buffer) > s.config.MaxBatchSize*2 {
			dropped := len(s.buffer) - s.config.MaxBatchSize*2
			s.buffer = s.buffer[:s.config.MaxBatchSize*2]
			s.logger.Warn("buffer overflow, dropping entries", zap.Int("dropped", dropped))
		}
		s.mu.Unlock()
		s.resetTimer()
		return err
	}

	// Publish opsnexus.log.ingested batch summary event (CloudEvents envelope)
	s.publishIngestedEvent(ctx, batch)

	s.resetTimer()
	return nil
}

// publishIngestedEvent 发布 opsnexus.log.ingested CloudEvents 批次摘要事件到 Kafka。
func (s *IngestService) publishIngestedEvent(ctx context.Context, batch []LogEntry) {
	if s.publisher == nil || len(batch) == 0 {
		return
	}

	levelsSummary := map[string]int{}
	var minTS, maxTS time.Time
	sourceHost, sourceService, sourceType := "", "", ""

	for i, e := range batch {
		levelsSummary[string(e.Level)]++
		if i == 0 || e.Timestamp.Before(minTS) {
			minTS = e.Timestamp
		}
		if i == 0 || e.Timestamp.After(maxTS) {
			maxTS = e.Timestamp
		}
		if sourceHost == "" {
			sourceHost = e.SourceHost
		}
		if sourceService == "" {
			sourceService = e.SourceService
		}
		if sourceType == "" {
			sourceType = e.SourceType
		}
	}

	now := time.Now().UTC()
	event := CloudEvent{
		SpecVersion:     "1.0",
		ID:              generateID(),
		Type:            "opsnexus.log.ingested",
		Source:          "/services/svc-log",
		Time:            now.Format(time.RFC3339),
		DataContentType: "application/json",
		Data: LogIngestedEventData{
			BatchID:       generateID(),
			SourceType:    sourceType,
			SourceHost:    sourceHost,
			SourceService: sourceService,
			LogCount:      len(batch),
			TimeRange: TimeRange{
				Start: &minTS,
				End:   &maxTS,
			},
			LevelsSummary: levelsSummary,
			ESIndex:       fmt.Sprintf("opsnexus-log-%s", now.Format("2006.01.02")),
		},
	}

	if err := s.publisher.Publish(ctx, "opsnexus.log.ingested", event); err != nil {
		s.logger.Error("failed to publish log.ingested event", zap.Error(err))
	}
}

// addToBuffer 将日志条目追加到内部缓冲区，达到批次上限时触发异步刷新。
func (s *IngestService) addToBuffer(entries []LogEntry) {
	s.mu.Lock()
	s.buffer = append(s.buffer, entries...)
	shouldFlush := len(s.buffer) >= s.config.MaxBatchSize
	s.mu.Unlock()

	if shouldFlush {
		go func() {
			if err := s.Flush(context.Background()); err != nil {
				s.logger.Error("buffer flush failed", zap.Error(err))
			}
		}()
	}
}

// resetTimer 重置定时刷新定时器。
func (s *IngestService) resetTimer() {
	s.timer.Reset(s.config.FlushInterval)
}

// validateEntry 校验日志条目必填字段，当前要求 message 不为空。
func validateEntry(entry *LogEntry) error {
	if entry.Message == "" {
		return fmt.Errorf("message is required")
	}
	return nil
}

// applyMasking 应用已配置的脱敏规则，对日志消息和标签值进行正则替换。
//
// BUG-003 修复：原始实现直接使用用户提供的正则模式进行全局替换，导致纯数字手机号模式
//（如 \d{11} 或 1[3-9]\d{9}）会匹配 18 位身份证号中的连续数字子串，产生误脱敏。
//
// 修复方案：对于模式最终匹配结果为纯数字序列的规则（通过判断 pattern 是否为纯数字
// 正则来识别），自动将其升级为边界感知模式——在原始捕获组外包裹非数字边界约束，
// 确保只匹配独立的数字串，而不匹配嵌入在更长数字串（如身份证号）中的子序列。
//
// Go 正则不支持 lookbehind，使用等效方案：
//   原模式 P → 安全模式 (?:^|[^\d])(P)(?:[^\d]|$)
//   替换时通过 ReplaceAllStringFunc + FindStringSubmatchIndex 只替换捕获组内容，
//   保留前后的非数字边界字符，避免截断相邻文本。
func (s *IngestService) applyMasking(entry *LogEntry, rules []*MaskingRule) {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// BUG-003 修复：检测是否为纯数字序列手机号模式（如 \d{11}、1[3-9]\d{9} 等）。
		// 这类模式的特征是：整体由数字字符类（\d、[0-9]、[3-9] 等）和量词组成，
		// 不含非数字锚定，因此容易误匹配嵌入在更长数字串中的子串。
		// 将其升级为带非数字边界约束的安全版本：要求匹配序列前后均不紧邻其他数字。
		if isNumericOnlyPattern(rule.Pattern) {
			// 将原始模式包裹为边界感知模式：
			//   (?:^|[^\d])  — 行首或非数字前缀（非捕获，不参与替换）
			//   (原始模式)    — 捕获组，仅替换此部分
			//   (?:[^\d]|$)  — 非数字后缀或行尾（非捕获，不参与替换）
			safePattern := `(?:^|[^\d])(` + rule.Pattern + `)(?:[^\d]|$)`
			re, err := regexp.Compile(safePattern)
			if err != nil {
				// 升级失败则回退到原始模式
				goto fallback
			}
			replacement := rule.Replacement
			// 使用 ReplaceAllStringFunc 逐匹配替换：通过 FindStringSubmatchIndex
			// 定位捕获组位置，仅替换数字部分，保留前后边界字符
			replFunc := func(match string) string {
				subIdx := re.FindStringSubmatchIndex(match)
				if len(subIdx) < 4 {
					return match
				}
				// subIdx[2]:subIdx[3] 是捕获组（数字序列）的字节范围
				return match[:subIdx[2]] + replacement + match[subIdx[3]:]
			}
			entry.Message = re.ReplaceAllStringFunc(entry.Message, replFunc)
			for k, v := range entry.Labels {
				entry.Labels[k] = re.ReplaceAllStringFunc(v, replFunc)
			}
			continue
		}

	fallback:
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		entry.Message = re.ReplaceAllString(entry.Message, rule.Replacement)
		for k, v := range entry.Labels {
			entry.Labels[k] = re.ReplaceAllString(v, rule.Replacement)
		}
	}
}

// isNumericOnlyPattern 判断正则模式是否为纯数字序列匹配模式。
// BUG-003 辅助函数：识别只匹配数字序列（手机号、身份证等）的模式，
// 用于决定是否需要自动添加非数字边界约束。
// 判断依据：模式中仅包含数字字符类（\d、[0-9]、[3-9] 等字符类）和量词，
// 不含字母、非数字锚定（如 \b、\s）或 ^ 之外的非数字元素。
func isNumericOnlyPattern(pattern string) bool {
	if pattern == "" {
		return false
	}
	// 仅含数字相关元素：\d、[数字字符类]、{量词}、* + ? 等重复符号
	// 通过尝试匹配一个典型的非数字字符串来判断：若模式能且只能匹配纯数字则为 true。
	// 简化实现：检测模式中是否只包含 \d、[0-9x]、量词和可选括号，
	// 不含 [a-z]、\w、\s、\b 等非数字元素。
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			// 转义序列：只允许 \d（数字）和 \D 之外的转义
			next := pattern[i+1]
			if next != 'd' && next != 'D' {
				// 允许 \d，不允许 \w、\s、\b 等非纯数字元素
				return false
			}
			i++ // 跳过转义字符的第二个字节
		case c == '[':
			// 字符类：找到匹配的 ] 并检查内容只含数字相关字符
			j := i + 1
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			cls := pattern[i+1 : j]
			for _, ch := range cls {
				// 字符类中只允许数字、-（范围）、^ （取反）、X/x（身份证后缀）
				if !('0' <= ch && ch <= '9') && ch != '-' && ch != '^' && ch != 'X' && ch != 'x' {
					return false
				}
			}
			i = j
		case c == '{' || c == '}' || c == '*' || c == '+' || c == '?' ||
			c == '(' || c == ')' || c == '|':
			// 量词和分组符号，允许
		case '0' <= c && c <= '9':
			// 量词数字，允许
		default:
			return false
		}
	}
	return true
}

// maskSensitiveFields 对标签键名和消息内容中匹配全局敏感字段列表的值进行掩码处理。
func (s *IngestService) maskSensitiveFields(entry *LogEntry) {
	for _, sensitive := range s.config.SensitiveFields {
		lower := strings.ToLower(sensitive)
		for k := range entry.Labels {
			if strings.Contains(strings.ToLower(k), lower) {
				entry.Labels[k] = "***MASKED***"
			}
		}
	}
	for _, sensitive := range s.config.SensitiveFields {
		lower := strings.ToLower(sensitive)
		if strings.Contains(strings.ToLower(entry.Message), lower+"=") {
			re := regexp.MustCompile(`(?i)(` + regexp.QuoteMeta(sensitive) + `)=\S+`)
			entry.Message = re.ReplaceAllString(entry.Message, "$1=***MASKED***")
		}
	}
}

// --- CRUD 委托方法：日志采集源、解析规则、脱敏规则、保留策略、日志流 ---

func (s *IngestService) CreateLogSource(ctx context.Context, src *LogSource) error {
	src.SourceID = generateID()
	src.CreatedAt = time.Now().UTC()
	src.UpdatedAt = src.CreatedAt
	return s.logRepo.CreateLogSource(ctx, src)
}

func (s *IngestService) GetLogSource(ctx context.Context, id string) (*LogSource, error) {
	return s.logRepo.GetLogSource(ctx, id)
}

func (s *IngestService) ListLogSources(ctx context.Context) ([]*LogSource, error) {
	return s.logRepo.ListLogSources(ctx)
}

func (s *IngestService) UpdateLogSource(ctx context.Context, src *LogSource) error {
	src.UpdatedAt = time.Now().UTC()
	return s.logRepo.UpdateLogSource(ctx, src)
}

func (s *IngestService) DeleteLogSource(ctx context.Context, id string) error {
	return s.logRepo.DeleteLogSource(ctx, id)
}

func (s *IngestService) CreateParseRule(ctx context.Context, rule *ParseRule) error {
	rule.RuleID = generateID()
	rule.CreatedAt = time.Now().UTC()
	return s.logRepo.CreateParseRule(ctx, rule)
}

func (s *IngestService) GetParseRule(ctx context.Context, id string) (*ParseRule, error) {
	return s.logRepo.GetParseRule(ctx, id)
}

func (s *IngestService) ListParseRules(ctx context.Context) ([]*ParseRule, error) {
	return s.logRepo.ListParseRules(ctx)
}

func (s *IngestService) UpdateParseRule(ctx context.Context, rule *ParseRule) error {
	return s.logRepo.UpdateParseRule(ctx, rule)
}

func (s *IngestService) DeleteParseRule(ctx context.Context, id string) error {
	return s.logRepo.DeleteParseRule(ctx, id)
}

func (s *IngestService) CreateMaskingRule(ctx context.Context, rule *MaskingRule) error {
	rule.RuleID = generateID()
	return s.logRepo.CreateMaskingRule(ctx, rule)
}

func (s *IngestService) GetMaskingRule(ctx context.Context, id string) (*MaskingRule, error) {
	return s.logRepo.GetMaskingRule(ctx, id)
}

func (s *IngestService) ListMaskingRules(ctx context.Context) ([]*MaskingRule, error) {
	return s.logRepo.ListMaskingRules(ctx)
}

func (s *IngestService) UpdateMaskingRule(ctx context.Context, rule *MaskingRule) error {
	return s.logRepo.UpdateMaskingRule(ctx, rule)
}

func (s *IngestService) DeleteMaskingRule(ctx context.Context, id string) error {
	return s.logRepo.DeleteMaskingRule(ctx, id)
}

func (s *IngestService) CreateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error {
	p.PolicyID = generateID()
	return s.logRepo.CreateRetentionPolicy(ctx, p)
}

func (s *IngestService) GetRetentionPolicy(ctx context.Context, id string) (*RetentionPolicy, error) {
	return s.logRepo.GetRetentionPolicy(ctx, id)
}

func (s *IngestService) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	return s.logRepo.ListRetentionPolicies(ctx)
}

func (s *IngestService) UpdateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error {
	return s.logRepo.UpdateRetentionPolicy(ctx, p)
}

func (s *IngestService) DeleteRetentionPolicy(ctx context.Context, id string) error {
	return s.logRepo.DeleteRetentionPolicy(ctx, id)
}

func (s *IngestService) CreateStream(ctx context.Context, stream *Stream) error {
	stream.ID = generateID()
	stream.CreatedAt = time.Now().UTC()
	if stream.RetentionDays == 0 {
		stream.RetentionDays = 30
	}
	return s.logRepo.CreateStream(ctx, stream)
}

func (s *IngestService) GetStream(ctx context.Context, id string) (*Stream, error) {
	return s.logRepo.GetStream(ctx, id)
}

func (s *IngestService) ListStreams(ctx context.Context, pageSize int, pageToken string) ([]*Stream, string, error) {
	return s.logRepo.ListStreams(ctx, pageSize, pageToken)
}

func (s *IngestService) DeleteStream(ctx context.Context, id string) error {
	return s.logRepo.DeleteStream(ctx, id)
}

// --- 辅助函数 ---

// generateID 基于当前纳秒时间戳生成唯一 ID。
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
