package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/opsnexus/svc-ai/internal/biz"
	"go.uber.org/zap"
)

func NewRouter(
	analysisUC *biz.AnalysisUseCase,
	modelRepo biz.ModelRepo,
	promptRepo biz.PromptRepo,
	logger *zap.Logger,
) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/v1/ai")
	{
		h := &analysisHandler{uc: analysisUC, logger: logger}
		api.POST("/analysis", h.createAnalysis)
		api.GET("/analysis", h.listAnalyses)
		api.GET("/analysis/:analysis_id", h.getAnalysis)
		api.POST("/analysis/:analysis_id/feedback", h.submitFeedback)

		mh := &modelHandler{repo: modelRepo, logger: logger}
		api.GET("/models", mh.listModels)

		ph := &promptHandler{repo: promptRepo, logger: logger}
		api.GET("/prompts", ph.listPrompts)

		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "svc-ai"})
		})
	}

	return r
}

type analysisHandler struct {
	uc     *biz.AnalysisUseCase
	logger *zap.Logger
}

func (h *analysisHandler) createAnalysis(c *gin.Context) {
	var req biz.AnalysisCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	task, err := h.uc.CreateAnalysis(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("create analysis failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, task)
}

func (h *analysisHandler) getAnalysis(c *gin.Context) {
	id, err := uuid.Parse(c.Param("analysis_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid analysis_id"})
		return
	}

	task, err := h.uc.GetAnalysis(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "analysis not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *analysisHandler) listAnalyses(c *gin.Context) {
	filter := biz.AnalysisFilter{
		PageToken: c.Query("page_token"),
		PageSize:  20,
	}

	if s := c.Query("status"); s != "" {
		status := biz.AnalysisStatus(s)
		filter.Status = &status
	}
	if t := c.Query("type"); t != "" {
		at := biz.AnalysisType(t)
		filter.Type = &at
	}

	tasks, nextToken, err := h.uc.ListAnalyses(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analyses":        tasks,
		"next_page_token": nextToken,
	})
}

func (h *analysisHandler) submitFeedback(c *gin.Context) {
	id, err := uuid.Parse(c.Param("analysis_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid analysis_id"})
		return
	}

	var req biz.FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	if err := h.uc.SubmitFeedback(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

type modelHandler struct {
	repo   biz.ModelRepo
	logger *zap.Logger
}

func (h *modelHandler) listModels(c *gin.Context) {
	models, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

type promptHandler struct {
	repo   biz.PromptRepo
	logger *zap.Logger
}

func (h *promptHandler) listPrompts(c *gin.Context) {
	prompts, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"prompts": prompts})
}
