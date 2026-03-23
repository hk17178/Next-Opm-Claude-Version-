package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/opsnexus/svc-notify/internal/biz"
	"go.uber.org/zap"
)

func NewRouter(
	notifyUC *biz.NotifyUseCase,
	botRepo biz.BotRepo,
	ruleRepo biz.BroadcastRuleRepo,
	channelManager *biz.ChannelManager,
	logger *zap.Logger,
) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/v1/notify")
	{
		nh := &notifyHandler{uc: notifyUC, logger: logger}
		api.POST("/send", nh.send)
		api.GET("/notifications", nh.listNotifications)
		api.GET("/notifications/:notification_id", nh.getNotification)

		ch := &channelHandler{repo: botRepo, channelManager: channelManager, logger: logger}
		api.GET("/channels", ch.listChannels)
		api.POST("/channels", ch.createChannel)
		api.PUT("/channels/:channel_id", ch.updateChannel)
		api.DELETE("/channels/:channel_id", ch.deleteChannel)
		api.POST("/channels/:channel_id/test", ch.testChannel)

		rh := &ruleHandler{repo: ruleRepo, logger: logger}
		api.GET("/broadcast-rules", rh.listRules)

		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "svc-notify"})
		})
	}

	return r
}

// --- Notification handlers ---

type notifyHandler struct {
	uc     *biz.NotifyUseCase
	logger *zap.Logger
}

func (h *notifyHandler) send(c *gin.Context) {
	var req biz.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	log, err := h.uc.Send(c.Request.Context(), req)
	if err != nil && log == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"notification_id": log.ID,
		"status":          log.Status,
	})
}

func (h *notifyHandler) getNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("notification_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid notification_id"})
		return
	}

	log, err := h.uc.GetNotification(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "notification not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, log)
}

func (h *notifyHandler) listNotifications(c *gin.Context) {
	filter := biz.NotificationFilter{
		PageToken: c.Query("page_token"),
		PageSize:  20,
	}

	if ch := c.Query("channel"); ch != "" {
		ct := biz.ChannelType(ch)
		filter.Channel = &ct
	}
	if s := c.Query("status"); s != "" {
		st := biz.NotifyStatus(s)
		filter.Status = &st
	}

	logs, nextToken, err := h.uc.ListNotifications(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications":   logs,
		"next_page_token": nextToken,
	})
}

// --- Channel handlers ---

// channelHandler 处理通知渠道（机器人）的 CRUD 和连通性测试。
type channelHandler struct {
	repo           biz.BotRepo          // 机器人数据仓储
	channelManager *biz.ChannelManager  // 渠道管理器，用于执行渠道连通性测试
	logger         *zap.Logger          // 日志记录器
}

func (h *channelHandler) listChannels(c *gin.Context) {
	bots, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channels": bots})
}

func (h *channelHandler) createChannel(c *gin.Context) {
	var req struct {
		Name   string          `json:"name" binding:"required"`
		Type   biz.ChannelType `json:"type" binding:"required"`
		Config json.RawMessage `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	bot := &biz.Bot{
		ID:          uuid.New(),
		Name:        req.Name,
		ChannelType: req.Type,
		Config:      req.Config,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.repo.Create(c.Request.Context(), bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bot)
}

func (h *channelHandler) updateChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("channel_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid channel_id"})
		return
	}

	bot, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "channel not found"})
		return
	}

	var req struct {
		Name    *string          `json:"name"`
		Enabled *bool            `json:"enabled"`
		Config  *json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	if req.Name != nil {
		bot.Name = *req.Name
	}
	if req.Enabled != nil {
		bot.Enabled = *req.Enabled
	}
	if req.Config != nil {
		bot.Config = *req.Config
	}

	if err := h.repo.Update(c.Request.Context(), bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bot)
}

func (h *channelHandler) deleteChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("channel_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid channel_id"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// testChannel 测试指定渠道的连通性。
// 调用 channelManager.Test() 向渠道发送测试消息，测试失败返回 502 Bad Gateway。
func (h *channelHandler) testChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("channel_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid channel_id"})
		return
	}

	// 查找对应的机器人配置
	bot, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "channel not found"})
		return
	}

	// 执行渠道连通性测试
	if err := h.channelManager.Test(c.Request.Context(), bot); err != nil {
		h.logger.Error("channel test failed",
			zap.String("channel_id", id.String()),
			zap.String("channel_type", string(bot.ChannelType)),
			zap.Error(err),
		)
		// 测试失败返回 502，表示上游渠道服务不可达
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    "TEST_FAILED",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "test_sent"})
}

// --- Broadcast rule handlers ---

type ruleHandler struct {
	repo   biz.BroadcastRuleRepo
	logger *zap.Logger
}

func (h *ruleHandler) listRules(c *gin.Context) {
	rules, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}
