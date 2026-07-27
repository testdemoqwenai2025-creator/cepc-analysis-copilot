package agents

import (
	"context"

	"github.com/cepc-analysis-copilot/internal/models"
	"github.com/google/uuid"
)

// Phase constants for the 6-phase analysis pipeline.
const (
	PhaseDataIngestion       = 1
	PhaseReconstruction      = 2
	PhaseSelectionSystematics = 3
	PhaseStatisticalAnalysis = 4
	PhaseVisualization       = 5
	PhaseLiteratureReview    = 6
)

// AgentInput is the universal input envelope for all agents.
type AgentInput struct {
	AnalysisID uuid.UUID      `json:"analysis_id"`
	Phase      int            `json:"phase"`
	Payload    map[string]any `json:"payload"`
}

// AgentOutput is the universal output envelope for all agents.
type AgentOutput struct {
	Success    bool           `json:"success"`
	AgentName  string         `json:"agent_name"`
	Phase      int            `json:"phase"`
	DurationMs int64          `json:"duration_ms"`
	Payload    map[string]any `json:"payload"`
	Error      string         `json:"error,omitempty"`
}

// Tool is the interface that every agent tool must implement.
// Tools are stateless functions with typed inputs and outputs.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
}

// Agent is the interface that every specialized agent must implement.
// Each agent handles one phase of the analysis pipeline (or is cross-cutting).
type Agent interface {
	// Name returns the human-readable agent identifier.
	Name() string

	// Phase returns the pipeline phase this agent is responsible for.
	// Cross-cutting agents return 0.
	Phase() int

	// Tools returns the list of tools this agent can invoke.
	Tools() []Tool

	// Execute runs the agent's primary task and returns structured output.
	Execute(ctx context.Context, input AgentInput) (*AgentOutput, error)

	// Verify checks the integrity of the agent's output before
	// the orchestrator passes it downstream.
	Verify(output *AgentOutput) error

	// FailureModes returns a list of known failure modes with
	// detection criteria and recovery strategies.
	FailureModes() []FailureMode
}

// FailureMode describes a known way the agent can fail.
type FailureMode struct {
	Name        string `json:"name"`
	Detection   string `json:"detection"`
	Recovery    string `json:"recovery"`
	IsRecoverable bool `json:"is_recoverable"`
}

// AnalysisState is the shared state object passed between agents
// through the orchestrator. No agent should hold a direct reference
// to another agent — all routing goes through the orchestrator.
type AnalysisState struct {
	Analysis    *models.Analysis `json:"analysis"`
	Dataset     *models.Dataset  `json:"dataset"`
	Papers      []models.Paper  `json:"papers"`
	Plots       []models.Plot   `json:"plots"`
	CurrentPhase int            `json:"current_phase"`
	Errors      []string       `json:"errors"`
	Warnings    []string       `json:"warnings"`
	Metadata    map[string]any `json:"metadata"`
}
