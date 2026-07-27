package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// VisualizationAgent handles Phase 5: Visualization & Plotting.
// It generates publication-quality physics plots with CEPC collaboration styling.
type VisualizationAgent struct {
	db  *gorm.DB
	cfg *config.Config
	log *zerolog.Logger
}

func NewVisualizationAgent(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *VisualizationAgent {
	return &VisualizationAgent{db: db, cfg: cfg, log: log}
}

func (a *VisualizationAgent) Name() string { return "Visualization Agent" }
func (a *VisualizationAgent) Phase() int   { return PhaseVisualization }

func (a *VisualizationAgent) Tools() []Tool {
	return []Tool{
		&CreateHistogramTool{cfg: a.cfg, log: a.log},
		&CreateContourPlotTool{cfg: a.cfg, log: a.log},
		&CreateEfficiencyMapTool{cfg: a.cfg, log: a.log},
		&ApplyCollaborationStyleTool{log: a.log},
		&ExportPlotlyTool{cfg: a.cfg, log: a.log},
		&CompareWithReferencePlotTool{cfg: a.cfg, log: a.log},
	}
}

func (a *VisualizationAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	start := time.Now()
	a.log.Info().Str("analysis_id", input.AnalysisID.String()).Msg("Visualization Agent: starting")

	plotRequests, _ := input.Payload["plot_requests"].([]map[string]any)
	plots := make([]map[string]any, 0, len(plotRequests))

	for _, req := range plotRequests {
		plotType, _ := req["type"].(string)
		var result map[string]any
		var err error

		switch plotType {
		case "stacked_histogram":
			result, err = a.Tools()[0].Execute(ctx, req)
		case "contour_plot":
			result, err = a.Tools()[1].Execute(ctx, req)
		case "efficiency_map":
			result, err = a.Tools()[2].Execute(ctx, req)
		default:
			a.log.Warn().Str("type", plotType).Msg("Unknown plot type, defaulting to histogram")
			result, err = a.Tools()[0].Execute(ctx, req)
		}

		if err != nil {
			a.log.Warn().Err(err).Str("type", plotType).Msg("Plot generation failed")
			continue
		}
		plots = append(plots, result)
	}

	durationMs := time.Since(start).Milliseconds()
	return &AgentOutput{
		Success:    true,
		AgentName:  a.Name(),
		Phase:      a.Phase(),
		DurationMs: durationMs,
		Payload: map[string]any{
			"plots":         plots,
			"total_created": len(plots),
		},
	}, nil
}

func (a *VisualizationAgent) Verify(output *AgentOutput) error {
	if !output.Success {
		return fmt.Errorf("agent reported failure: %s", output.Error)
	}
	plots, ok := output.Payload["plots"].([]map[string]any)
	if !ok || len(plots) == 0 {
		return fmt.Errorf("at least one plot must be generated")
	}
	return nil
}

func (a *VisualizationAgent) FailureModes() []FailureMode {
	return []FailureMode{
		{Name: "empty_dataset", Detection: "Histogram has 0 entries in all bins", Recovery: "Return placeholder with 'No data' annotation", IsRecoverable: true},
		{Name: "bin_range_mismatch", Detection: "Datasets have incompatible axis ranges", Recovery: "Auto-normalize ranges to union", IsRecoverable: true},
		{Name: "font_rendering_failure", Detection: "Matplotlib falls back to tofu boxes", Recovery: "Verify Noto Sans SC; use ASCII fallback", IsRecoverable: true},
		{Name: "plotly_export_failure", Detection: "Plotly HTML generation error", Recovery: "Return static PNG only", IsRecoverable: true},
	}
}

// --- Tool stubs ---

type CreateHistogramTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *CreateHistogramTool) Name() string { return "create_histogram" }
func (t *CreateHistogramTool) Description() string { return "Generates 1D/2D histograms with configurable binning and error bars" }
func (t *CreateHistogramTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"file_path": "/cache/plots/histogram.png", "dpi": 300}, nil
}

type CreateContourPlotTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *CreateContourPlotTool) Name() string { return "create_contour_plot" }
func (t *CreateContourPlotTool) Description() string { return "2D exclusion contours in parameter space" }
func (t *CreateContourPlotTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"file_path": "/cache/plots/contour.png", "dpi": 300}, nil
}

type CreateEfficiencyMapTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *CreateEfficiencyMapTool) Name() string { return "create_efficiency_map" }
func (t *CreateEfficiencyMapTool) Description() string { return "Binned efficiency or purity plots as 2D heatmaps" }
func (t *CreateEfficiencyMapTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"file_path": "/cache/plots/efficiency_map.png", "dpi": 300}, nil
}

type ApplyCollaborationStyleTool struct {
	log *zerolog.Logger
}
func (t *ApplyCollaborationStyleTool) Name() string { return "apply_collaboration_style" }
func (t *ApplyCollaborationStyleTool) Description() string { return "Applies CEPC/CERN-approved color palette, font sizes, and layout" }
func (t *ApplyCollaborationStyleTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"style_applied": "cepc_collaboration"}, nil
}

type ExportPlotlyTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *ExportPlotlyTool) Name() string { return "export_plotly" }
func (t *ExportPlotlyTool) Description() string { return "Creates interactive HTML version using Plotly" }
func (t *ExportPlotlyTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"interactive_url": "/plots/interactive.html"}, nil
}

type CompareWithReferencePlotTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *CompareWithReferencePlotTool) Name() string { return "compare_with_reference_plot" }
func (t *CompareWithReferencePlotTool) Description() string { return "Overlays published results on the same axes" }
func (t *CompareWithReferencePlotTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"overlaid": true}, nil
}
