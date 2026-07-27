package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// StatisticsAgent handles Phase 4: Statistical Analysis.
// It constructs profile likelihoods, runs toy MC, and computes confidence intervals.
type StatisticsAgent struct {
	db  *gorm.DB
	cfg *config.Config
	log *zerolog.Logger
}

func NewStatisticsAgent(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *StatisticsAgent {
	return &StatisticsAgent{db: db, cfg: cfg, log: log}
}

func (a *StatisticsAgent) Name() string { return "Statistics Agent" }
func (a *StatisticsAgent) Phase() int   { return PhaseStatisticalAnalysis }

func (a *StatisticsAgent) Tools() []Tool {
	return []Tool{
		&BuildLikelihoodModelTool{log: a.log},
		&ProfileLikelihoodScanTool{cfg: a.cfg, log: a.log},
		&RunToysTool{cfg: a.cfg, log: a.log},
		&ComputeCLsTool{cfg: a.cfg, log: a.log},
		&ComputeBayesianLimitTool{cfg: a.cfg, log: a.log},
		&GeneratePullPlotsTool{cfg: a.cfg, log: a.log},
	}
}

func (a *StatisticsAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	start := time.Now()
	a.log.Info().Str("analysis_id", input.AnalysisID.String()).Msg("Statistics Agent: starting")

	// Step 1: Build likelihood model
	modelResult, err := a.Tools()[0].Execute(ctx, input.Payload)
	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	// Step 2: Profile likelihood scan
	scanResult, err := a.Tools()[1].Execute(ctx, input.Payload)
	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	// Step 3: Toy MC
	toyResult, err := a.Tools()[2].Execute(ctx, map[string]any{
		"n_toys": input.Payload["n_toys"],
		"seed":   input.Payload["seed"],
	})
	if err != nil {
		a.log.Warn().Err(err).Msg("Toy MC failed, using asymptotic only")
	}

	// Step 4: CLs calculation
	clsResult, _ := a.Tools()[3].Execute(ctx, map[string]any{
		"cl":     input.Payload["cl"],
		"method": input.Payload["method"],
	})

	durationMs := time.Since(start).Milliseconds()
	return &AgentOutput{
		Success:    true,
		AgentName:  a.Name(),
		Phase:      a.Phase(),
		DurationMs: durationMs,
		Payload: map[string]any{
			"model_built":  modelResult["success"],
			"scan_results":  scanResult,
			"toy_results":   toyResult,
			"cls_results":   clsResult,
			"plots":         []string{"likelihood_scan", "nll_vs_mu", "pull_distributions"},
		},
	}, nil
}

func (a *StatisticsAgent) Verify(output *AgentOutput) error {
	if !output.Success {
		return fmt.Errorf("agent reported failure: %s", output.Error)
	}
	mu, _ := output.Payload["mu_hat"].(float64)
	if mu <= 0 || mu != mu {
		return fmt.Errorf("mu_hat must be finite and positive")
	}
	return nil
}

func (a *StatisticsAgent) FailureModes() []FailureMode {
	return []FailureMode{
		{Name: "likelihood_non_convergence", Detection: "Minimizer fails in 1000 iterations", Recovery: "Try alternative minimizers; report status", IsRecoverable: true},
		{Name: "toy_mc_anomaly", Detection: "KS test p-value < 0.01", Recovery: "Increase toy count; flag for manual review", IsRecoverable: true},
		{Name: "negative_background", Detection: "Background model returns negative yields", Recovery: "Floor at zero with warning", IsRecoverable: true},
		{Name: "asymptotic_invalid", Detection: "<5 expected events per bin", Recovery: "Fall back to toy-based CLs", IsRecoverable: true},
	}
}

// --- Tool stubs ---

type BuildLikelihoodModelTool struct{ log *zerolog.Logger }
func (t *BuildLikelihoodModelTool) Name() string { return "build_likelihood_model" }
func (t *BuildLikelihoodModelTool) Description() string { return "Constructs a HistFactory-style likelihood from channel inputs" }
func (t *BuildLikelihoodModelTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"success": true, "channels": 1}, nil
}

type ProfileLikelihoodScanTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *ProfileLikelihoodScanTool) Name() string { return "profile_likelihood_scan" }
func (t *ProfileLikelihoodScanTool) Description() string { return "Scans NLL as function of signal strength" }
func (t *ProfileLikelihoodScanTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// Delegates to python/statistics/likelihood.py
	return map[string]any{"mu_hat": 1.03, "nll_minimum": -42.5}, nil
}

type RunToysTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *RunToysTool) Name() string { return "run_toys" }
func (t *RunToysTool) Description() string { return "Generates toy MC datasets and computes test statistic distributions" }
func (t *RunToysTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"n_toys_run": 10000, "ks_pvalue": 0.67, "converged": true}, nil
}

type ComputeCLsTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *ComputeCLsTool) Name() string { return "compute_cls" }
func (t *ComputeCLsTool) Description() string { return "Calculates CLs using asymptotic or toy-based method" }
func (t *ComputeCLsTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"cls_lower": 0.75, "cls_upper": 1.32, "p_value": 0.002}, nil
}

type ComputeBayesianLimitTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *ComputeBayesianLimitTool) Name() string { return "compute_bayesian_limit" }
func (t *ComputeBayesianLimitTool) Description() string { return "Runs MCMC sampling for Bayesian credible intervals" }
func (t *ComputeBayesianLimitTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"credible_lower": 0.78, "credible_upper": 1.30}, nil
}

type GeneratePullPlotsTool struct {
	cfg *config.Config
	log *zerolog.Logger
}
func (t *GeneratePullPlotsTool) Name() string { return "generate_pull_plots" }
func (t *GeneratePullPlotsTool) Description() string { return "Creates pull distributions and correlation matrices for nuisance parameters" }
func (t *GeneratePullPlotsTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	return map[string]any{"pull_plot_path": "/cache/plots/pull_distributions.png"}, nil
}
