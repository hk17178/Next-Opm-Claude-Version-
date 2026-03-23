package biz

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"
)

// KnowledgeRepo defines the data access interface for knowledge base operations (PostgreSQL).
type KnowledgeRepo interface {
	Create(ctx context.Context, article *KnowledgeArticle) error
	Get(ctx context.Context, id string) (*KnowledgeArticle, error)
	Update(ctx context.Context, article *KnowledgeArticle) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, articleType string, page, pageSize int) ([]*KnowledgeArticle, int, error)
	SearchByVector(ctx context.Context, embedding []float32, limit int, threshold float64) ([]*KnowledgeSearchResult, error)
	SearchByText(ctx context.Context, query string, limit int) ([]*KnowledgeSearchResult, error)
}

// EmbeddingService generates vector embeddings from text (FR-17-003).
type EmbeddingService interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// KnowledgeUsecase implements knowledge base operations with vector search.
// FR-17-001 ~ FR-17-008
type KnowledgeUsecase struct {
	repo      KnowledgeRepo
	embedder  EmbeddingService
	threshold float64
	logger    *zap.Logger
}

// NewKnowledgeUsecase creates a new knowledge use case.
func NewKnowledgeUsecase(repo KnowledgeRepo, embedder EmbeddingService, threshold float64, logger *zap.Logger) *KnowledgeUsecase {
	return &KnowledgeUsecase{
		repo:      repo,
		embedder:  embedder,
		threshold: threshold,
		logger:    logger,
	}
}

// Create adds a new knowledge article and generates its embedding.
// FR-17-001: AI generates case draft after incident closure.
func (uc *KnowledgeUsecase) Create(ctx context.Context, article *KnowledgeArticle) error {
	if article.Title == "" || article.Content == "" {
		return fmt.Errorf("title and content are required")
	}
	if article.Type == "" {
		article.Type = KnowledgeTypeCaseStudy
	}
	if article.Status == "" {
		article.Status = KnowledgeStatusDraft
	}

	if uc.embedder != nil {
		embedding, err := uc.embedder.Embed(ctx, article.Title+" "+article.Content)
		if err != nil {
			uc.logger.Warn("failed to generate embedding", zap.Error(err))
		} else {
			article.Embedding = embedding
		}
	}

	return uc.repo.Create(ctx, article)
}

// Get returns a knowledge article by ID.
func (uc *KnowledgeUsecase) Get(ctx context.Context, id string) (*KnowledgeArticle, error) {
	return uc.repo.Get(ctx, id)
}

// Update modifies an existing knowledge article and regenerates its embedding.
func (uc *KnowledgeUsecase) Update(ctx context.Context, article *KnowledgeArticle) error {
	if uc.embedder != nil {
		embedding, err := uc.embedder.Embed(ctx, article.Title+" "+article.Content)
		if err != nil {
			uc.logger.Warn("failed to regenerate embedding", zap.Error(err))
		} else {
			article.Embedding = embedding
		}
	}
	return uc.repo.Update(ctx, article)
}

// Delete removes a knowledge article.
func (uc *KnowledgeUsecase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}

// List returns paginated knowledge articles, optionally filtered by type (FR-17-002).
func (uc *KnowledgeUsecase) List(ctx context.Context, articleType string, page, pageSize int) ([]*KnowledgeArticle, int, error) {
	return uc.repo.List(ctx, articleType, page, pageSize)
}

// Search performs semantic search using vector similarity with cosine distance (FR-17-003).
// The repo layer uses pgvector's `<=>` operator for cosine similarity.
// Falls back to text-based search if embedding generation fails.
// Results are sorted by quality_score (descending) as a secondary rank after similarity.
// FR-17-004: Used by alert handler to find similar cases.
func (uc *KnowledgeUsecase) Search(ctx context.Context, query string, limit int) ([]*KnowledgeSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []*KnowledgeSearchResult
	var err error

	if uc.embedder != nil {
		embedding, err := uc.embedder.Embed(ctx, query)
		if err != nil {
			uc.logger.Warn("embedding failed, falling back to text search", zap.Error(err))
			results, err = uc.repo.SearchByText(ctx, query, limit)
			if err != nil {
				return nil, err
			}
		} else {
			results, err = uc.repo.SearchByVector(ctx, embedding, limit, uc.threshold)
			if err != nil {
				return nil, err
			}
		}
	} else {
		results, err = uc.repo.SearchByText(ctx, query, limit)
		if err != nil {
			return nil, err
		}
	}

	// Sort results: primary by similarity (desc), secondary by quality_score (desc).
	// This ensures higher-quality articles surface when similarity scores are close.
	sortByRelevanceAndQuality(results)

	return results, nil
}

// SearchByIncident finds knowledge articles related to a specific incident (FR-17-004).
// Filters results to only include articles with a matching related_incident field,
// then performs semantic search within that subset.
func (uc *KnowledgeUsecase) SearchByIncident(ctx context.Context, query string, incidentID string, limit int) ([]*KnowledgeSearchResult, error) {
	if incidentID == "" {
		return nil, fmt.Errorf("incident_id is required for incident-scoped search")
	}
	if limit <= 0 {
		limit = 10
	}

	// Perform full search first, then filter by related_incident.
	// A more efficient implementation would push this filter into the repo layer.
	allResults, err := uc.Search(ctx, query, limit*3) // over-fetch to account for filtering
	if err != nil {
		return nil, err
	}

	var filtered []*KnowledgeSearchResult
	for _, r := range allResults {
		if r.Article.RelatedIncident != nil && *r.Article.RelatedIncident == incidentID {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// sortByRelevanceAndQuality sorts search results by similarity (desc), breaking ties
// with quality_score (desc). This implements FR-17-006 quality scoring integration.
func sortByRelevanceAndQuality(results []*KnowledgeSearchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Similarity != results[j].Similarity {
			return results[i].Similarity > results[j].Similarity
		}
		return results[i].Article.QualityScore > results[j].Article.QualityScore
	})
}
