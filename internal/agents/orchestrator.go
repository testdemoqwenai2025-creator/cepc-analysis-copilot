package agents

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/cepc-analysis-copilot/internal/config"
    "github.com/cepc-analysis-copilot/internal/models"
    "github.com/google/uuid"
    "github.com/rs/zerolog"
    "gorm.io/gorm"
)

// Orchestrator manages the 6-phase analysis pipeline.
// It routes tasks to agents, collects results, and persists
// state after each phase for crash recovery.
type Orchestrator struct {
    db     *gorm.DB
    cfg    *config.Config
    log    *zerolog.Logger
    agents map[int]Agent // phase -> agent mapping
}

// NewOrchestrator creates an orchestrator with all 6 agents registered.
func NewOrchestrator(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *Orchestrator {
    o := &Orchestrator{db: db, cfg: cfg, log: log}

    // Register phase-based agents
    dataAgent := NewDataAgent(db, cfg, log)
    analysisAgent := NewAnalysisAgent(db, cfg, log)
    statsAgent := NewStatisticsAgent(db, cfg, log)
    vizAgent := NewVisualizationAgent(db, cfg, log)
    reviewAgent := NewReviewAgent(db, cfg, log)

    o.agents = map[int]Agent{
        PhaseDataIngestion:        dataAgent,
        PhaseSelectionSystematics: analysisAgent,
        PhaseStatisticalAnalysis:  statsAgent,
        PhaseVisualization:        vizAgent,
        PhaseLiteratureReview:     reviewAgent,
    }

    return o
}

// LiteratureAgent accessor for cross-cutting use.
func (o *Orchestrator) LiteratureAgent() *LiteratureAgent {
    return NewLiteratureAgent(o.db, o.cfg, o.log)
}

// RunPipeline executes the full 6-phase analysis pipeline.
// It can resume from a checkpoint if a previous run crashed.
func (o *Orchestrator) RunPipeline(ctx context.Context, analysisID uuid.UUID, state *AnalysisState) (*AnalysisState, error) {
    o.log.Info().
        Str("analysis_id", analysisID.String()).
        Int("start_phase", state.CurrentPhase+1).
        Msg("Orchestrator: starting pipeline")

    // Phase ordering for the sequential pipeline
    phaseOrder := []int{
        PhaseDataIngestion,
        PhaseSelectionSystematics,
        PhaseStatisticalAnalysis,
        PhaseVisualization,
        PhaseLiteratureReview,
    }

    for _, phase := range phaseOrder {
        if phase <= state.CurrentPhase {
            o.log.Debug().Int("phase", phase).Msg("Skipping completed phase")
            continue
        }

        agent, exists := o.agents[phase]
        if !exists {
            o.log.Warn().Int("phase", phase).Msg("No agent registered for phase")
            continue
        }

        // Execute the agent
        input := AgentInput{
            AnalysisID: analysisID,
            Phase:      phase,
            Payload:    o.buildPayload(phase, state),
        }

        output, err := agent.Execute(ctx, input)
        if err != nil {
            state.Errors = append(state.Errors,
                fmt.Sprintf("Phase %d (%s): %s", phase, agent.Name(), err.Error()))

            // Check if failure is recoverable
            if !o.isRecoverable(agent, err) {
                return state, fmt.Errorf("unrecoverable failure in phase %d (%s): %w",
                    phase, agent.Name(), err)
            }

            state.Warnings = append(state.Warnings,
                fmt.Sprintf("Phase %d (%s) had a recoverable error: %s", phase, agent.Name(), err.Error()))
            continue
        }

        // Verify output
        if verifyErr := agent.Verify(output); verifyErr != nil {
            state.Warnings = append(state.Warnings,
                fmt.Sprintf("Phase %d verification failed: %s", phase, verifyErr.Error()))
        }

        // Update state
        state.CurrentPhase = phase
        o.mergeOutput(state, output)

        // Persist checkpoint
        if err := o.persistCheckpoint(analysisID, phase, state); err != nil {
            o.log.Warn().Err(err).Int("phase", phase).Msg("Failed to persist checkpoint")
        }

        // Log agent execution
        o.logAgentExecution(analysisID, phase, agent.Name(), output)

        o.log.Info().
            Int("phase", phase).
            Str("agent", agent.Name()).
            Int64("duration_ms", output.DurationMs).
            Bool("success", output.Success).
            Msg("Phase complete")
    }

    o.log.Info().
        Str("analysis_id", analysisID.String()).
        Int("final_phase", state.CurrentPhase).
        Int("errors", len(state.Errors)).
        Int("warnings", len(state.Warnings)).
        Msg("Pipeline complete")

    return state, nil
}

// RunPhaseConcurrent executes specific phases in parallel using goroutines.
// Useful for e.g., running Literature Agent in parallel with Analysis Agent.
func (o *Orchestrator) RunPhaseConcurrent(ctx context.Context, analysisID uuid.UUID, phases []int, state *AnalysisState) (*AnalysisState, error) {
    var wg sync.WaitGroup
    var mu sync.Mutex
    errs := make([]error, len(phases))

    for i, phase := range phases {
        agent, exists := o.agents[phase]
        if !exists {
            errs[i] = fmt.Errorf("no agent for phase %d", phase)
            continue
        }

        wg.Add(1)
        go func(idx int, p int, a Agent) {
            defer wg.Done()

            input := AgentInput{
                AnalysisID: analysisID,
                Phase:      p,
                Payload:    o.buildPayload(p, state),
            }

            output, err := a.Execute(ctx, input)
            if err != nil {
                mu.Lock()
                errs[idx] = err
                mu.Unlock()
                return
            }

            _ = a.Verify(output) // Best-effort verification in concurrent mode

            mu.Lock()
            state.CurrentPhase = p
            o.mergeOutput(state, output)
            mu.Unlock()
        }(i, phase, agent)
    }

    wg.Wait()

    for _, e := range errs {
        if e != nil {
            return state, e
        }
    }

    return state, nil
}

// buildPayload constructs the input payload for a given phase from the current state.
func (o *Orchestrator) buildPayload(phase int, state *AnalysisState) map[string]any {
    payload := make(map[string]any)

    switch phase {
    case PhaseDataIngestion:
        if state.Dataset != nil {
            payload["dataset_name"] = state.Dataset.Name
            payload["format"] = state.Dataset.Format
            payload["file_paths"] = []string{state.Dataset.FilePath}
        }
    case PhaseSelectionSystematics:
        if state.Dataset != nil {
            payload["storage_path"] = state.Dataset.StoragePath
        }
        payload["cuts"] = []map[string]any{
            {"variable": "Electron_pt", "min": 10, "unit": "GeV"},
            {"variable": "Electron_eta", "max": 2.5},
            {"variable": "Z1_mass", "min": 60, "max": 120, "unit": "GeV"},
            {"variable": "Higgs_mass", "min": 110, "max": 160, "unit": "GeV"},
        }
        payload["systematics"] = []string{"jet_energy_scale", "btag_efficiency", "lepton_id"}
    case PhaseStatisticalAnalysis:
        payload["n_toys"] = 10000
        payload["seed"] = 42
        payload["cl"] = 0.95
        payload["method"] = "cls"
    case PhaseVisualization:
        payload["plot_requests"] = []map[string]any{
            {
                "type":  "stacked_histogram",
                "title": "4-lepton invariant mass",
                "style": "cepc_collaboration",
            },
        }
    case PhaseLiteratureReview:
        payload["generate_analysis_note"] = true
    }

    return payload
}

// mergeOutput merges agent output back into the shared state.
func (o *Orchestrator) mergeOutput(state *AnalysisState, output *AgentOutput) {
    for k, v := range output.Payload {
        state.Metadata[k] = v
    }
}

// isRecoverable checks if a failure in a specific agent is recoverable.
func (o *Orchestrator) isRecoverable(agent Agent, err error) bool {
    for _, fm := range agent.FailureModes() {
        if fm.IsRecoverable {
            return true
        }
    }
    return false
}

// persistCheckpoint saves the current pipeline state to the database.
func (o *Orchestrator) persistCheckpoint(analysisID uuid.UUID, phase int, state *AnalysisState) error {
    step := models.AnalysisStep{
        AnalysisID: analysisID,
        Phase:      phase,
        Status:     "completed",
        DurationMs: time.Since(time.Now()).Milliseconds(), // Placeholder
    }
    return o.db.Create(&step).Error
}

// logAgentExecution records an agent action in the database.
func (o *Orchestrator) logAgentExecution(analysisID uuid.UUID, phase int, agentName string, output *AgentOutput) {
    log := models.AgentLog{
        AgentID:   agentName,
        Phase:     phase,
        DurationMs: output.DurationMs,
        Status:    map[bool]string{true: "success", false: "failed"}[output.Success],
        Message:   fmt.Sprintf("Phase %d completed", phase),
    }
    if output.Error != "" {
        log.Message = output.Error
    }
    o.db.Create(&log)
}