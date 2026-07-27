package db

import (
	"fmt"
	"time"

	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgresPool creates a new GORM database connection and runs auto-migrations.
func NewPostgresPool(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Info
	if cfg.Mode == "release" {
		logLevel = logger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Connection pool settings tuned for concurrent agent workloads
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Auto-migrate all models
	if err := db.AutoMigrate(
		&models.Dataset{},
		&models.Analysis{},
		&models.AnalysisStep{},
		&models.Paper{},
		&models.Plot{},
		&models.AgentLog{},
	); err != nil {
		return nil, fmt.Errorf("failed to run auto-migrations: %w", err)
	}

	// Enable pgvector extension for literature embeddings
	enablePgvector(db)

	return db, nil
}

func enablePgvector(db *gorm.DB) {
	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
}
