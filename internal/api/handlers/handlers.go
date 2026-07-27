package handlers

import (
	"net/http"
	"time"

	"github.com/cepc-analysis-copilot/internal/agents"
	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Handlers holds references to the database, config, and orchestrator.
type Handlers struct {
	DB          *gorm.DB
	Cfg         *config.Config
	Orchestrator *agents.Orchestrator
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db *gorm.DB, cfg *config.Config, orch *agents.Orchestrator) *Handlers {
	return &Handlers{DB: db, Cfg: cfg, Orchestrator: orch}
}

// HealthCheck returns the service health status.
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "cepc-analysis-copilot",
		"timestamp": time.Now().UTC(),
	})
}

// --- Dataset Handlers ---

func (h *Handlers) ListDatasets(c *gin.Context) {
	var datasets []models.Dataset
	if err := h.DB.Order("created_at DESC").Find(&datasets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"datasets": datasets, "count": len(datasets)})
}

func (h *Handlers) GetDataset(c *gin.Context) {
	id := c.Param("id")
	var dataset models.Dataset
	if err := h.DB.Where("dataset_id = ?", id).First(&dataset).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dataset not found"})
		return
	}
	c.JSON(http.StatusOK, dataset)
}

// IngestDataset starts the data ingestion pipeline (Phase 1).
func (h *Handlers) IngestDataset(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
		Format   string `json:"format"`
		Name     string `json:"name" binding:"required"`
		Process  string `json:"process"`
		Energy   string `json:"energy"`
		Generator string `json:"generator"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysisID := uuid.New()
	state := &agents.AnalysisState{
		Dataset: &models.Dataset{
			Name:      req.Name,
			Format:    req.Format,
			FilePath:  req.FilePath,
			Process:   req.Process,
			Energy:    req.Energy,
			Generator: req.Generator,
		},
		Metadata: make(map[string]any),
	}

	// Run Phase 1 (data ingestion) via orchestrator
	result, err := h.Orchestrator.RunPipeline(c.Request.Context(), analysisID, state)
	if err != nil {
		log.Error().Err(err).Msg("Data ingestion failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":    err.Error(),
			"warnings": result.Warnings,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Dataset ingested successfully",
		"dataset_id": result.Metadata["dataset_id"],
		"event_count": result.Metadata["event_count"],
		"warnings":   result.Warnings,
	})
}

func (h *Handlers) DeleteDataset(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Where("dataset_id = ?", id).Delete(&models.Dataset{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "dataset deleted"})
}

// --- Analysis Handlers ---

func (h *Handlers) ListAnalyses(c *gin.Context) {
	var analyses []models.Analysis
	if err := h.DB.Order("created_at DESC").Find(&analyses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": analyses, "count": len(analyses)})
}

func (h *Handlers) GetAnalysis(c *gin.Context) {
	id := c.Param("id")
	var analysis models.Analysis
	if err := h.DB.Preload("Steps").Preload("Plots").Where("analysis_id = ?", id).First(&analysis).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
		return
	}
	c.JSON(http.StatusOK, analysis)
}

func (h *Handlers) CreateAnalysis(c *gin.Context) {
	var req struct {
		DatasetID string   `json:"dataset_id" binding:"required"`
		Name      string   `json:"name" binding:"required"`
		Channel   string   `json:"channel"`
		Cuts      []map[string]any `json:"cuts"`
		Systematics []string `json:"systematics"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analysisID := uuid.New()
	analysis := &models.Analysis{
		AnalysisID: analysisID.String(),
		Name:       req.Name,
		Channel:    req.Channel,
		Status:     "created",
	}
	if err := h.DB.Create(analysis).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"analysis_id": analysisID.String()})
}

// RunAnalysis triggers the full 6-phase pipeline for an analysis.
func (h *Handlers) RunAnalysis(c *gin.Context) {
	id := c.Param("id")
	var analysis models.Analysis
	if err := h.DB.Where("analysis_id = ?", id).First(&analysis).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
		return
	}

	// Update status to running
	h.DB.Model(&analysis).Updates(map[string]any{"status": "running"})

	// Run the full pipeline in a goroutine (async)
	go func() {
		state := &agents.AnalysisState{
			Analysis: &analysis,
			Metadata: make(map[string]any),
		}
		uid, _ := uuid.Parse(analysis.AnalysisID)
		result, err := h.Orchestrator.RunPipeline(c.Request.Context(), uid, state)

		status := "completed"
		if err != nil {
			status = "failed"
			log.Error().Err(err).Str("analysis_id", id).Msg("Pipeline failed")
		}
		if len(result.Errors) > 0 {
			status = "completed_with_errors"
		}
		h.DB.Model(&analysis).Updates(map[string]any{
			"status":      status,
			"current_phase": state.CurrentPhase,
		})
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Pipeline started",
		"analysis_id": id,
	})
}

func (h *Handlers) GetAnalysisSteps(c *gin.Context) {
	id := c.Param("id")
	var steps []models.AnalysisStep
	h.DB.Where("analysis_id = ?", id).Order("phase ASC").Find(&steps)
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// --- Literature Handlers ---

func (h *Handlers) SearchPapers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	litAgent := h.Orchestrator.LiteratureAgent()
	analysisID := uuid.New()
	output, err := litAgent.Execute(c.Request.Context(), agents.AgentInput{
		AnalysisID: analysisID,
		Payload: map[string]any{
			"query_type": "search",
			"query_text": query,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output.Payload)
}

func (h *Handlers) ListPapers(c *gin.Context) {
	var papers []models.Paper
	h.DB.Order("published_date DESC").Find(&papers)
	c.JSON(http.StatusOK, gin.H{"papers": papers, "count": len(papers)})
}

func (h *Handlers) GetPaper(c *gin.Context) {
	id := c.Param("id")
	var paper models.Paper
	if err := h.DB.Where("arxiv_id = ?", id).First(&paper).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "paper not found"})
		return
	}
	c.JSON(http.StatusOK, paper)
}

func (h *Handlers) TriggerMonitor(c *gin.Context) {
	// Trigger a manual arXiv monitor run
	litAgent := h.Orchestrator.LiteratureAgent()
	analysisID := uuid.New()
	output, err := litAgent.Execute(c.Request.Context(), agents.AgentInput{
		AnalysisID: analysisID,
		Payload: map[string]any{
			"query_type": "monitor",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output.Payload)
}

// --- Plot Handlers ---

func (h *Handlers) ServePlot(c *gin.Context) {
	id := c.Param("id")
	var plot models.Plot
	if err := h.DB.First(&plot, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plot not found"})
		return
	}
	c.File(plot.FilePath)
}

// --- Log Handlers ---

func (h *Handlers) GetAgentLogs(c *gin.Context) {
	var logs []models.AgentLog
	query := h.DB.Order("created_at DESC").Limit(100)

	if agentID := c.Query("agent"); agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}

	query.Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}