package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/models"
	"github.com/cepc-analysis-copilot/pkg/utils"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// DataAgent handles Phase 1: Data Ingestion & Validation.
// It reads ROOT/nanoAOD/Parquet files, validates schemas, computes
// quality metrics, and stores metadata in PostgreSQL.
type DataAgent struct {
	db    *gorm.DB
	cfg   *config.Config
	log   *zerolog.Logger
}

// NewDataAgent creates a new DataAgent instance.
func NewDataAgent(db *gorm.DB, cfg *config.Config, log *zerolog.Logger) *DataAgent {
	return &DataAgent{db: db, cfg: cfg, log: log}
}

func (a *DataAgent) Name() string { return "Data Agent" }
func (a *DataAgent) Phase() int   { return PhaseDataIngestion }

func (a *DataAgent) Tools() []Tool {
	return []Tool{
		&ReadRootFileTool{cfg: a.cfg, log: a.log},
		&ValidateSchemaTool{log: a.log},
		&ConvertToParquetTool{cfg: a.cfg, log: a.log},
		&ComputeDataQualityTool{cfg: a.cfg, log: a.log},
		&RegisterDatasetTool{db: a.db, log: a.log},
	}
}

// Execute ingests a physics data file and returns validated dataset metadata.
func (a *DataAgent) Execute(ctx context.Context, input AgentInput) (*AgentOutput, error) {
	start := time.Now()
	a.log.Info().Str("analysis_id", input.AnalysisID.String()).Msg("Data Agent: starting ingestion")

	filePaths, ok := input.Payload["file_paths"].([]string)
	if !ok || len(filePaths) == 0 {
		return nil, fmt.Errorf("file_paths is required and must be a non-empty string slice")
	}
	filePath := filePaths[0] // Primary file

	format, _ := input.Payload["format"].(string)
	if format == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".root":
			format = "root"
		case ".parquet":
			format = "parquet"
		default:
			return nil, fmt.Errorf("unsupported file format: %s", ext)
		}
	}

	// Step 1: Read file and extract metadata
	readResult, err := a.Tools()[0].Execute(ctx, map[string]any{
		"file_path": filePath,
		"format":    format,
	})
	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	// Step 2: Validate schema
	schemaResult, err := a.Tools()[1].Execute(ctx, map[string]any{
		"branches":  readResult["branches"],
		"format":   format,
	})
	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	// Step 3: Convert to Parquet for fast access (if ROOT input)
	var storagePath string
	if format == "root" {
		convResult, err := a.Tools()[2].Execute(ctx, map[string]any{
			"file_path": filePath,
			"output_dir": a.cfg.CacheDir,
		})
		if err != nil {
			return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
		}
		storagePath = convResult["storage_path"].(string)
	} else {
		storagePath = filePath
	}

	// Step 4: Compute data quality metrics
	qualityResult, err := a.Tools()[4].Execute(ctx, map[string]any{
		"file_path": storagePath,
		"format":    "parquet",
	})
	if err != nil {
		a.log.Warn().Err(err).Msg("Data quality check had issues, continuing")
	}

	// Step 5: Register dataset in database
	datasetID := fmt.Sprintf("ds_%s_%s", utils.Slugify(input.Payload["dataset_name"]), utils.ShortUUID())
	regResult, err := a.Tools()[4].Execute(ctx, map[string]any{
		"dataset_id": datasetID,
		"name":       input.Payload["dataset_name"],
		"format":     format,
		"file_path":  filePath,
		"storage_path": storagePath,
		"event_count": readResult["event_count"],
		"branches":   readResult["branches"],
		"schema_hash": schemaResult["schema_hash"],
		"quality_flags": qualityResult["quality_flags"],
	})
	if err != nil {
		return &AgentOutput{Success: false, AgentName: a.Name(), Phase: a.Phase(), Error: err.Error()}, err
	}

	durationMs := time.Since(start).Milliseconds()
	a.log.Info().
		Str("dataset_id", datasetID).
		Int64("duration_ms", durationMs).
		Msg("Data Agent: ingestion complete")

	return &AgentOutput{
		Success:    true,
		AgentName:  a.Name(),
		Phase:      a.Phase(),
		DurationMs: durationMs,
		Payload: map[string]any{
			"dataset_id":     datasetID,
			"event_count":    readResult["event_count"],
			"branches":       readResult["branches"],
			"schema_hash":    schemaResult["schema_hash"],
			"quality_flags":  qualityResult["quality_flags"],
			"storage_path":   storagePath,
			"registered_id":  regResult["db_id"],
		},
	}, nil
}

// Verify checks that the Data Agent's output meets all integrity requirements.
func (a *DataAgent) Verify(output *AgentOutput) error {
	if !output.Success {
		return fmt.Errorf("agent reported failure: %s", output.Error)
	}
	p := output.Payload

	if p["event_count"] == nil || p["event_count"].(int64) <= 0 {
		return fmt.Errorf("event_count must be positive")
	}
	if p["dataset_id"] == nil || p["dataset_id"].(string) == "" {
		return fmt.Errorf("dataset_id is required")
	}
	if p["schema_hash"] == nil || p["schema_hash"].(string) == "" {
		return fmt.Errorf("schema_hash is required")
	}

	qf, ok := p["quality_flags"].(models.QualityFlags)
	if ok && qf.HasNaN {
		return fmt.Errorf("data quality check failed: NaN values detected in strict mode")
	}

	return nil
}

// FailureModes returns the known failure modes for the Data Agent.
func (a *DataAgent) FailureModes() []FailureMode {
	return []FailureMode{
		{
			Name:           "file_not_found",
			Detection:      "read_root_file returns I/O error",
			Recovery:       "Report to user with file path; suggest re-upload",
			IsRecoverable: false,
		},
		{
			Name:           "schema_mismatch",
			Detection:      "validate_schema fails branch check",
			Recovery:       "Log missing branches; offer partial ingestion in lenient mode",
			IsRecoverable: true,
		},
		{
			Name:           "out_of_memory",
			Detection:      "Python process OOM or JS heap limit",
			Recovery:       "Switch to chunked reading (100k events/batch)",
			IsRecoverable: true,
		},
		{
			Name:           "duplicate_dataset",
			Detection:      "register_dataset finds matching schema_hash",
			Recovery:       "Return existing dataset_id; skip re-ingestion",
			IsRecoverable: true,
		},
	}
}

// --- Tool Implementations ---

type ReadRootFileTool struct {
	cfg *config.Config
	log *zerolog.Logger
}

func (t *ReadRootFileTool) Name() string        { return "read_root_file" }
func (t *ReadRootFileTool) Description() string { return "Opens ROOT TTree/TNtuple, reads branch metadata and data ranges" }

func (t *ReadRootFileTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	filePath := input["file_path"].(string)

	// Delegate to Python for ROOT file reading via uproot
	script := filepath.Join(t.cfg.PythonPath, "io/root_reader.py")
	cmd := exec.CommandContext(ctx, "python3", script, "inspect", filePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("python root_reader failed: %w (output: %s)", err, string(out))
	}

	// Parse Python JSON output
	result, err := utils.ParseJSON(out)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root_reader output: %w", err)
	}

	return result, nil
}

type ValidateSchemaTool struct {
	log *zerolog.Logger
}

func (t *ValidateSchemaTool) Name() string        { return "validate_schema" }
func (t *ValidateSchemaTool) Description() string { return "Checks branch types, naming conventions, and required physics objects" }

func (t *ValidateSchemaTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	branches, _ := input["branches"].([]string)

	// Required physics object branches for CEPC analyses
	required := []string{"Electron_pt", "Electron_eta", "Jet_m", "MissingET_met"}
	missing := []string{}
	for _, r := range required {
		found := false
		for _, b := range branches {
			if b == r {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, r)
		}
	}

	schemaHash := utils.HashStrings(branches)

	result := map[string]any{
		"schema_hash":      schemaHash,
		"branches_validated": len(branches),
		"missing_required":  missing,
	}
	if len(missing) > 0 {
		result["warnings"] = fmt.Sprintf("missing required branches: %v", missing)
	}

	return result, nil
}

type ConvertToParquetTool struct {
	cfg *config.Config
	log *zerolog.Logger
}

func (t *ConvertToParquetTool) Name() string        { return "convert_to_parquet" }
func (t *ConvertToParquetTool) Description() string { return "Converts ROOT trees to Apache Parquet for fast columnar access" }

func (t *ConvertToParquetTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	filePath := input["file_path"].(string)
	outputDir := input["output_dir"].(string)

	os.MkdirAll(outputDir, 0755)
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	outPath := filepath.Join(outputDir, baseName+".parquet")

	script := filepath.Join(t.cfg.PythonPath, "io/root_reader.py")
	cmd := exec.CommandContext(ctx, "python3", script, "convert", filePath, "--output", outPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("parquet conversion failed: %w (output: %s)", err, string(out))
	}

	return map[string]any{"storage_path": outPath, "converted": true}, nil
}

type ComputeDataQualityTool struct {
	cfg *config.Config
	log *zerolog.Logger
}

func (t *ComputeDataQualityTool) Name() string        { return "compute_data_quality" }
func (t *ComputeDataQualityTool) Description() string { return "Calculates event yields, NaN fractions, luminosity cross-checks" }

func (t *ComputeDataQualityTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	filePath := input["file_path"].(string)

	script := filepath.Join(t.cfg.PythonPath, "analysis/quality.py")
	cmd := exec.CommandContext(ctx, "python3", script, filePath)
	out, err := cmd.Output()
	if err != nil {
		return map[string]any{
			"quality_flags": models.QualityFlags{
				HasNaN:               true,
				LuminosityConsistent: false,
			},
			"warning": "quality check script failed, assuming dirty data",
		}, nil
	}

	result, err := utils.ParseJSON(out)
	if err != nil {
		return map[string]any{
			"quality_flags": models.QualityFlags{HasNaN: true, LuminosityConsistent: false},
			"warning": "could not parse quality output",
		}, nil
	}

	return result, nil
}

type RegisterDatasetTool struct {
	db  *gorm.DB
	log *zerolog.Logger
}

func (t *RegisterDatasetTool) Name() string        { return "register_dataset" }
func (t *RegisterDatasetTool) Description() string { return "Writes dataset metadata to PostgreSQL" }

func (t *RegisterDatasetTool) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	// This would create a models.Dataset record and persist it.
	// For now, return a placeholder ID.
	dbID := uuid.New()

	return map[string]any{
		"db_id":      dbID.String(),
		"registered": true,
	}, nil
}
