package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/models"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// ReviewAgent handles Phase 6: Validation & Peer Check.
// It validates all agent outputs against consistency checks,
// literature benchmarks, and collaboration standards.
type ReviewAgent struct {
	db  *gorm.DB
	cfg *config.Config
	log *zerolog.Logger
}

func NewReviewAgent(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *ReviewAgent {
	return &ReviewAgent{db: db, cfg: cfg, log: log}
}

func (a *ReviewAgent) Name() string { return "Review Agent" }
func (a *ReviewAgent) Phase() int   { return PhaseLiteratureReview }

func (a *ReviewAgent) Tools() []Tool {
	return []Tool{
		&CheckYieldConsistencyTool{log: a.log},
		&CheckStatisticalValidityTool{log: a.log},
		&CrossReferenceLiteratureTool{db: a.db, log: a.log},
		&CheckSystematicCoverageTool{log: a.log},
		&ValidatePlotStandardsTool{log: a.log},
		&DraftAnalysisNoteTool{cfg: a.cfg, log: a.log},
	}
}

// Execute runs all review checks and produces a verdict.
func (a *ReviewAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	start := time.Now()
	a.log.Info().Str("analysis_id", input.AnalysisID.String()).Msg("Review Agent: starting review")

	checks := make([]models.ReviewCheck, 0)
	var recommendations []string
	allPassed := true

	// Run each review check
	for i, tool := range a.Tools() {
		result, err := tool.Execute(ctx, input.Payload)
		if err != nil {
			checks = append(checks, models.ReviewCheck{
				Check:    tool.Name(),
				Passed:   false,
				Severity: "error",
				Detail:   err.Error(),
			})
			allPassed = false
			continue
		}

		passed, _ := result["passed"].(bool)
		severity, _ := result["severity"].(string)
		detail, _ := result["detail"].(string)
		if severity == "" {
			severity = "info"
		}

		checks = append(checks, models.ReviewCheck{
			Check:    tool.Name(),
			Passed:   passed,
			Severity: severity,
			Detail:   detail,
		})

		if !passed {
			allPassed = false
			if rec, ok := result["recommendation"].(string); ok && rec != "" {
				recommendations = append(recommendations, rec)
			}
		}
		_ = i // avoid unused
	}

	verdict := "pass"
	if !allPassed {
		verdict = "pass_with_warnings"
		for _, c := range checks {
			if !c.Passed && c.Severity == "error" {
				verdict = "fail"
				break
			}
		}
	}

	// Generate analysis note draft if requested
	var notePath string
	if genNote, _ := input.Payload["generate_analysis_note"].(bool); genNote {
		noteResult, _ := a.Tools()[5].Execute(ctx, input.Payload)
		notePath, _ = noteResult["note_path"].(string)
	}

	durationMs := time.Since(start).Milliseconds()
	return &AgentOutput{
		Success:    true,
		AgentName:  a.Name(),
		Phase:      a.Phase(),
		DurationMs: durationMs,
		Payload: map[string]any{
			"review_checks":  checks,
			"overall_verdict": verdict,
			"recommendations": recommendations,
			"analysis_note": notePath,
		},
	}, nil
}

func (a *ReviewAgent) Verify(output *AgentOutput) error {
	if !output.Success {
		return fmt.Errorf("agent reported failure: %s", output.Error)
	}
	verdict, ok := output.Payload["overall_verdict"].(string)
	validVerdicts := map[string]bool{"pass": true, "pass_with_warnings": true, "fail": true}
	if !ok || !validVerdicts[verdict] {
		return fmt.Errorf("invalid verdict: %s", verdict)
	}
	return nil
}

func (a *ReviewAgent) FailureModes() []FailureMode {
	return []FailureMode{
		{Name: "critical_inconsistency", Detection: "Review check returns severity=error", Recovery: "Block finalization; require user acknowledgment", IsRecoverable: false},
		{Name: "literature_comparison_unavailable", Detection: "No matching reference analysis found", Recovery: "Flag as info; proceed without external validation", IsRecoverable: true},
		{Name: "note_generation_failure", Detection: "LaTeX compilation or missing template", Recovery: "Return plain-text summary instead", IsRecoverable: true},
	}
}

// --- Tool stubs ---

type CheckYieldConsistencyTool struct{ log *zerolog.Logger }
func (t *CheckYieldConsistencyTool) Name() string { return "check_yield_consistency" }
func (t *CheckYieldConsistencyTool) Description() string { return "Verifies cut flow is monotonically decreasing" }
func (t *CheckYieldConsistencyTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"passed": true, "detail": "Cut flow yields are monotonically decreasing."}, nil
}

type CheckStatisticalValidityTool struct{ log *zerolog.Logger }
func (t *CheckStatisticalValidityTool) Name() string { return "check_statistical_validity" }
func (t *CheckStatisticalValidityTool) Description() string { return "Validates convergence, toy agreement, uncertainty propagation" }
func (t *CheckStatisticalValidityTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"passed": true, "detail": "Profile likelihood converged. Toy MC agrees."}, nil
}

type CrossReferenceLiteratureTool struct {
	db  *gorm.DB
	log *zerolog.Logger
}
func (t *CrossReferenceLiteratureTool) Name() string { return "cross_reference_literature" }
func (t *CrossReferenceLiteratureTool) Description() string { return "Compares results against published measurements" }
func (t *CrossReferenceLiteratureTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{
		"passed":        false,
		"severity":      "warning",
		"detail":        "Measured mu differs from CEPC pre-study projection by ~2 sigma.",
		"recommendation": "Verify luminosity normalization against official CEPC MC campaign numbers",
	}, nil
}

type CheckSystematicCoverageTool struct{ log *zerolog.Logger }
func (t *CheckSystematicCoverageTool) Name() string { return "check_systematic_coverage" }
func (t *CheckSystematicCoverageTool) Description() string { return "Ensures all relevant systematics are included" }
func (t *CheckSystematicCoverageTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{
		"passed":   true,
		"detail":   "All requested systematics evaluated. Consider adding ISR/FSR.",
		"recommendation": "Consider adding ISR/FSR systematic (identified in recent literature)",
	}, nil
}

type ValidatePlotStandardsTool struct{ log *zerolog.Logger }
func (t *ValidatePlotStandardsTool) Name() string { return "validate_plot_standards" }
func (t *ValidatePlotStandardsTool) Description() string { return "Checks plot dimensions, fonts, colors against collaboration templates" }
func (t *ValidatePlotStandardsTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"passed": true, "detail": "All plots meet CEPC collaboration formatting requirements."}, nil
}

type DraftAnalysisNoteTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *DraftAnalysisNoteTool) Name() string { return "draft_analysis_note" }
func (t *DraftAnalysisNoteTool) Description() string { return "Generates a LaTeX analysis note template populated with results" }
func (t *DraftAnalysisNoteTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"note_path": "/cache/analysis/notes/AN_v1.tex"}, nil
}
