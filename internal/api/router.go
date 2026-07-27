package api

import (
	"net/http"

	"github.com/cepc-analysis-copilot/internal/api/handlers"
	"github.com/cepc-analysis-copilot/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(h *handlers.Handlers) *gin.Engine {
	r := gin.Default()

	// Middleware
	r.Use(middleware.CORS())
	r.Use(middleware.RequestLogger())

	// Health check
	r.GET("/api/health", h.HealthCheck)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Datasets
		datasets := v1.Group("/datasets")
		{
			datasets.GET("", h.ListDatasets)
			datasets.GET("/:id", h.GetDataset)
			datasets.POST("/ingest", h.IngestDataset)
			datasets.DELETE("/:id", h.DeleteDataset)
		}

		// Analyses
		analyses := v1.Group("/analyses")
		{
			analyses.GET("", h.ListAnalyses)
			analyses.GET("/:id", h.GetAnalysis)
			analyses.POST("", h.CreateAnalysis)
			analyses.POST("/:id/run", h.RunAnalysis)
			analyses.GET("/:id/steps", h.GetAnalysisSteps)
		}

		// Literature
		literature := v1.Group("/literature")
		{
			literature.GET("/search", h.SearchPapers)
			literature.GET("/papers", h.ListPapers)
			literature.GET("/papers/:id", h.GetPaper)
			literature.POST("/monitor", h.TriggerMonitor)
		}

		// Plots
		v1.GET("/plots/:id", h.ServePlot)

		// Agent logs
		v1.GET("/logs", h.GetAgentLogs)
	}

	// Static files (HTMX templates)
	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "./web/static")

	// Page routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "CEPC Analysis Copilot",
		})
	})
	r.GET("/analyses/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "analysis.html", gin.H{
			"title": "Analyses",
		})
	})
	r.GET("/analyses/:id", func(c *gin.Context) {
		c.HTML(http.StatusOK, "analysis.html", gin.H{
			"title": "Analysis",
		})
	})
	r.GET("/literature", func(c *gin.Context) {
		c.HTML(http.StatusOK, "literature.html", gin.H{
			"title": "Literature",
		})
	})

	return r
}