package models

import (
	"time"

	"github.com/google/uuid"
)

// Dataset represents an ingested physics data file (ROOT, nanoAOD, or Parquet).
type Dataset struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DatasetID      string         `gorm:"uniqueIndex;not null" json:"dataset_id"`
	Name           string         `gorm:"not null" json:"name"`
	Format         string         `gorm:"not null" json:"format"`
	FilePath       string         `gorm:"not null" json:"file_path"`
	StoragePath    string         `json:"storage_path"`
	EventCount     int64          `json:"event_count"`
	Branches       []string       `gorm:"type:text[]" json:"branches"`
	SchemaHash     string         `gorm:"index" json:"schema_hash"`
	QualityFlags   QualityFlags   `gorm:"type:jsonb" json:"quality_flags"`
	Process        string         `json:"process"`
	Energy         string         `json:"energy"`
	Generator      string         `json:"generator"`
	FileSizeBytes  int64          `json:"file_size_bytes"`
	Status         string         `gorm:"default:'pending'" json:"status"`
	ErrorMessage   string         `json:"error_message"`
	Analyses       []Analysis     `json:"analyses,omitempty"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// QualityFlags holds data quality check results.
type QualityFlags struct {
	HasNaN                bool    `json:"has_nan"`
	HasNegativeEnergy     bool    `json:"has_negative_energy"`
	LuminosityConsistent  bool    `json:"luminosity_consistent"`
	NaNFraction           float64 `json:"nan_fraction"`
	DuplicateEventFraction float64 `json:"duplicate_event_fraction"`
}

// Analysis represents a full 6-phase analysis run.
type Analysis struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AnalysisID       string         `gorm:"uniqueIndex;not null" json:"analysis_id"`
	DatasetID        uuid.UUID      `gorm:"type:uuid;index" json:"dataset_id"`
	Dataset          Dataset        `json:"dataset,omitempty"`
	Name             string         `gorm:"not null" json:"name"`
	Description      string         `json:"description"`
	Channel          string         `gorm:"index" json:"channel"`
	Energy           string         `json:"energy"`
	CutFlow          []CutFlowStep  `gorm:"type:jsonb" json:"cut_flow"`
	Significance     float64        `json:"significance"`
	SystematicSummary SystematicSummary `gorm:"type:jsonb" json:"systematic_summary"`
	StatResults      *StatResults   `gorm:"type:jsonb" json:"stat_results"`
	ReviewVerdict    string         `json:"review_verdict"`
	ReviewChecks     []ReviewCheck  `gorm:"type:jsonb" json:"review_checks"`
	Recommendations  []string       `gorm:"type:text[]" json:"recommendations"`
	Status           string         `gorm:"default:'pending'" json:"status"`
	CurrentPhase     int            `gorm:"default:0" json:"current_phase"`
	ErrorMessage     string         `json:"error_message"`
	Steps            []AnalysisStep `json:"steps,omitempty"`
	Plots            []Plot         `json:"plots,omitempty"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// CutFlowStep represents one step in the analysis cut flow.
type CutFlowStep struct {
	Step       string  `json:"step"`
	Events     int64   `json:"events"`
	Efficiency float64 `json:"efficiency"`
}

// SystematicSummary holds the results of systematic uncertainty studies.
type SystematicSummary struct {
	Systematics map[string]SystematicEffect `json:"systematics"`
}

// SystematicEffect represents the impact of a single systematic uncertainty.
type SystematicEffect struct {
	Plus  string `json:"plus"`
	Minus string `json:"minus"`
	Detail string `json:"detail"`
}

// StatResults holds the output of the Statistics Agent.
type StatResults struct {
	MuHat           float64         `json:"mu_hat"`
	UncertaintyPlus  float64         `json:"uncertainty_plus"`
	UncertaintyMinus float64         `json:"uncertainty_minus"`
	ConfidenceInterval ConfidenceInterval `json:"confidence_interval"`
	ExpectedLimit    *ExpectedLimit   `json:"expected_limit"`
	PValue           float64         `json:"p_value"`
	SignificanceSigma float64        `json:"significance_sigma"`
	ToyAgreement     *ToyAgreement   `json:"toy_agreement"`
}

// ConfidenceInterval holds upper/lower bounds.
type ConfidenceInterval struct {
	CL    float64 `json:"cl"`
	Method string  `json:"method"`
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

// ExpectedLimit holds expected exclusion limits with sigma bands.
type ExpectedLimit struct {
	Median     float64 `json:"median"`
	Plus1Sigma float64 `json:"plus_1sigma"`
	Minus1Sigma float64 `json:"minus_1sigma"`
	Plus2Sigma float64 `json:"plus_2sigma"`
}

// ToyAgreement holds toy MC validation results.
type ToyAgreement struct {
	KSTestPValue float64 `json:"ks_test_pvalue"`
	Converged    bool    `json:"converged"`
}

// ReviewCheck represents a single review check result.
type ReviewCheck struct {
	Check    string `json:"check"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

// AnalysisStep logs each agent's execution within a pipeline run.
type AnalysisStep struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AnalysisID uuid.UUID      `gorm:"type:uuid;index;not null" json:"analysis_id"`
	Phase      int            `gorm:"not null" json:"phase"`
	AgentName  string         `gorm:"index" json:"agent_name"`
	Status     string         `json:"status"`
	InputHash  string         `json:"input_hash"`
	OutputHash string         `json:"output_hash"`
	DurationMs int64          `json:"duration_ms"`
	Output     string         `gorm:"type:jsonb" json:"output"`
	Error      string         `json:"error"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// Paper represents a literature paper ingested from arXiv or INSPIRE-HEP.
type Paper struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ArxivID       string     `gorm:"uniqueIndex" json:"arxiv_id"`
	InspireID     string     `gorm:"index" json:"inspire_id"`
	Title         string     `gorm:"not null" json:"title"`
	Authors       []string   `gorm:"type:text[]" json:"authors"`
	Abstract      string     `json:"abstract"`
	Categories    []string   `gorm:"type:text[]" json:"categories"`
	PublishedDate time.Time  `json:"published_date"`
	Summary       string     `json:"summary"`
	KeyResults    []string   `gorm:"type:text[]" json:"key_results"`
	Embedding     []float32  `gorm:"type:vector(1536)" json:"-"`
	RelevanceScores map[string]float64 `gorm:"type:jsonb" json:"relevance_scores"`
	ApplicableSystematics []string `gorm:"type:text[]" json:"applicable_systematics"`
	IsBookmarked   bool       `gorm:"default:false" json:"is_bookmarked"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// Plot represents a generated visualization.
type Plot struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AnalysisID  uuid.UUID `gorm:"type:uuid;index" json:"analysis_id"`
	Analysis    Analysis   `json:"analysis,omitempty"`
	Title       string     `gorm:"not null" json:"title"`
	PlotType    string     `json:"plot_type"`
	FilePath    string     `json:"file_path"`
	FileFormat  string     `json:"file_format"`
	InteractiveURL string   `json:"interactive_url"`
	ConfigJSON  string     `gorm:"type:jsonb" json:"config_json"`
	DPI         int        `json:"dpi"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// AgentLog records every agent action for audit and debugging.
type AgentLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AgentID    string    `gorm:"index" json:"agent_id"`
	Phase      int       `json:"phase"`
	ToolName   string    `json:"tool_name"`
	InputHash  string    `json:"input_hash"`
	OutputHash string    `json:"output_hash"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
