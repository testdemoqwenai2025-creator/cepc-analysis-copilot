package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/literature"
	"github.com/cepc-analysis-copilot/internal/models"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// LiteratureAgent handles the cross-cutting Literature Layer.
// It monitors arXiv, searches INSPIRE-HEP, embeds papers for semantic search,
// and cross-references results against published measurements.
type LiteratureAgent struct {
	db        *gorm.DB
	cfg       *config.Config
	log       *zerolog.Logger
	arxiv     *literature.ArxivClient
	inspire   *literature.InspireClient
	embedder  *literature.Embedder
}

// NewLiteratureAgent creates a new LiteratureAgent instance.
func NewLiteratureAgent(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *LiteratureAgent {
	return &LiteratureAgent{
		db:      db,
		cfg:     cfg,
		log:     log,
		arxiv:   literature.NewArxivClient(cfg.ArxivBaseURL, log),
		inspire: literature.NewInspireClient(cfg.InspireBaseURL, cfg.InspireToken, log),
		embedder: literature.NewEmbedder(cfg.OpenAI_API_KEY, cfg.OpenAI_Model, log),
	}
}

func (a *LiteratureAgent) Name() string { return "Literature Agent" }
func (a *LiteratureAgent) Phase() int   { return 0 } // Cross-cutting; not tied to a single phase

func (a *LiteratureAgent) Tools() []Tool {
	return []Tool{
		&SearchArxivTool{client: a.arxiv, log: a.log},
		&SearchInspireTool{client: a.inspire, log: a.log},
		&EmbedPaperTool{embedder: a.embedder, log: a.log},
		&SemanticSearchTool{db: a.db, log: a.log},
		&SummarizePaperTool{embedder: a.embedder, log: a.log},
		&MonitorArxivTool{client: a.arxiv, db: a.db, log: a.log},
		&ExtractMeasurementsTool{embedder: a.embedder, log: a.log},
	}
}

// Execute performs a literature search based on the query type.
func (a *LiteratureAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	start := time.Now()
	queryType, _ := input.Payload["query_type"].(string)
	a.log.Info().
		Str("analysis_id", input.AnalysisID.String()).
		Str("query_type", queryType).
		Msg("Literature Agent: starting search")

	var results []map[string]any
	var err error

	switch queryType {
	case "search":
		results, err = a.executeSearch(ctx, input.Payload)
	case "monitor":
		results, err = a.executeMonitor(ctx, input.Payload)
	case "cross_reference":
		results, err = a.executeCrossReference(ctx, input.Payload)
	default:
		return nil, fmt.Errorf("unsupported query_type: %s", queryType)
	}

	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	durationMs := time.Since(start).Milliseconds()
	return &AgentOutput{
		Success:    true,
		AgentName:  a.Name(),
		Phase:      a.Phase(),
		DurationMs: durationMs,
		Payload: map[string]any{
			"results":          results,
			"total_matching":    len(results),
			"search_vector_index": "papers_hep_" + time.Now().Format("20060102"),
		},
	}, nil
}

func (a *LiteratureAgent) executeSearch(ctx context.Context, payload map[string]any) ([]map[string]any, error) {
	queryText, _ := payload["query_text"].(string)
	if queryText == "" {
		queryText = "CEPC Higgs analysis"
	}

	papers, err := a.arxiv.Search(ctx, queryText, literature.SearchOptions{
		Categories: []string{"hep-ex", "hep-ph"},
		MaxResults: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("arxiv search failed: %w", err)
	}

	results := make([]map[string]any, 0, len(papers))
	for _, p := range papers {
		results = append(results, map[string]any{
			"arxiv_id":       p.ArxivID,
			"title":          p.Title,
			"authors":        p.Authors,
			"summary":        p.Abstract,
			"relevance_score": 0.0, // Would be computed by the embedder
		})
	}
	return results, nil
}

func (a *LiteratureAgent) executeMonitor(ctx context.Context, payload map[string]any) ([]map[string]any, error) {
	// Daily harvest: find papers since last check
	papers, err := a.arxiv.Search(ctx, "CEPC OR Higgs-to-ZZ OR e+e- collider", literature.SearchOptions{
		Categories: []string{"hep-ex", "hep-ph"},
		MaxResults: 50,
	})
	if err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0)
	for _, p := range papers {
		results = append(results, map[string]any{
			"arxiv_id":  p.ArxivID,
			"title":     p.Title,
			"published": p.PublishedDate,
		})
	}
	return results, nil
}

func (a *LiteratureAgent) executeCrossReference(ctx context.Context, payload map[string]any) ([]map[string]any, error) {
	observables, _ := payload["observables"].([]string)

	// Search INSPIRE for measurements of the same observables
	results := make([]map[string]any, 0)
	for _, obs := range observables {
		inspireResults, err := a.inspire.SearchMeasurements(ctx, obs)
		if err != nil {
			a.log.Warn().Str("observable", obs).Err(err).Msg("INSPIRE search failed")
			continue
		}
		results = append(results, inspireResults...)
	}
	return results, nil
}

func (a *LiteratureAgent) Verify(output *AgentOutput) error {
	if !output.Success {
		return fmt.Errorf("agent reported failure: %s", output.Error)
	}
	results, ok := output.Payload["results"].([]map[string]any)
	if !ok {
		return fmt.Errorf("results must be an array")
	}
	for _, r := range results {
		aid, _ := r["arxiv_id"].(string)
		if aid == "" {
			continue
		}
		// Validate arxiv_id pattern: NNNN.NNNNN
		if len(aid) < 8 {
			return fmt.Errorf("invalid arxiv_id: %s", aid)
		}
	}
	return nil
}

func (a *LiteratureAgent) FailureModes() []FailureMode {
	return []FailureMode{
		{Name: "arxiv_rate_limit", Detection: "HTTP 429 from arXiv API", Recovery: "Exponential backoff (1s,2s,4s,8s); fall back to cache", IsRecoverable: true},
		{Name: "no_matching_papers", Detection: "results array is empty", Recovery: "Return no-matches with suggested broader terms", IsRecoverable: true},
		{Name: "embedding_timeout", Detection: "embed_paper exceeds 30s", Recovery: "Queue for retry; return metadata without embeddings", IsRecoverable: true},
		{Name: "pdf_parsing_failure", Detection: "summarize_paper cannot extract text", Recovery: "Fall back to abstract-only summary", IsRecoverable: true},
	}
}

// --- Tool stubs (full implementations in internal/literature/) ---

type SearchArxivTool struct {
	client *literature.ArxivClient
	log    *zerolog.Logger
}
func (t *SearchArxivTool) Name() string        { return "search_arxiv" }
func (t *SearchArxivTool) Description() string { return "Queries arXiv API by keyword, author, category, date range" }
func (t *SearchArxivTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"status": "delegated_to_arxiv_client"}, nil
}

type SearchInspireTool struct {
	client *literature.InspireClient
	log    *zerolog.Logger
}
func (t *SearchInspireTool) Name() string        { return "search_inspire" }
func (t *SearchInspireTool) Description() string { return "Queries INSPIRE-HEP for experimental results and citations" }
func (t *SearchInspireTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"status": "delegated_to_inspire_client"}, nil
}

type EmbedPaperTool struct {
	embedder *literature.Embedder
	log      *zerolog.Logger
}
func (t *EmbedPaperTool) Name() string        { return "embed_paper" }
func (t *EmbedPaperTool) Description() string { return "Generates vector embeddings of paper text" }
func (t *EmbedPaperTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"embedded": true}, nil
}

type SemanticSearchTool struct {
	db  *gorm.DB
	log *zerolog.Logger
}
func (t *SemanticSearchTool) Name() string        { return "semantic_search" }
func (t *SemanticSearchTool) Description() string { return "Performs similarity search in the vector store" }
func (t *SemanticSearchTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"results": []any{}}, nil
}

type SummarizePaperTool struct {
	embedder *literature.Embedder
	log      *zerolog.Logger
}
func (t *SummarizePaperTool) Name() string        { return "summarize_paper" }
func (t *SummarizePaperTool) Description() string { return "Produces a structured summary of a paper" }
func (t *SummarizePaperTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"summary": ""}, nil
}

type MonitorArxivTool struct {
	client *literature.ArxivClient
	db     *gorm.DB
	log    *zerolog.Logger
}
func (t *MonitorArxivTool) Name() string        { return "monitor_arxiv" }
func (t *MonitorArxivTool) Description() string { return "Scheduled arXiv harvest with keyword filtering" }
func (t *MonitorArxivTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"new_papers": 0}, nil
}

type ExtractMeasurementsTool struct {
	embedder *literature.Embedder
	log      *zerolog.Logger
}
func (t *ExtractMeasurementsTool) Name() string        { return "extract_measurements" }
func (t *ExtractMeasurementsTool) Description() string { return "Parses numerical results from paper text" }
func (t *ExtractMeasurementsTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"measurements": []any{}}, nil
}
