package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cepc-analysis-copilot/internal/agents"
	"github.com/cepc-analysis-copilot/internal/api"
	"github.com/cepc-analysis-copilot/internal/api/handlers"
	"github.com/cepc-analysis-copilot/internal/config"
	"github.com/cepc-analysis-copilot/internal/db"
	"github.com/cepc-analysis-copilot/pkg/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	utils.InitRandomSeed()

	// Logging setup
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	log.Info().Str("mode", cfg.Mode).Str("port", cfg.Port).Msg("Configuration loaded")

	// Connect to database
	database, err := db.NewPostgresPool(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	log.Info().Msg("Database connected and migrated")

	// Create orchestrator and agent system
	orch := agents.NewOrchestrator(database, cfg, &log)
	log.Info().Msg("Agent orchestrator initialized")

	// Create HTTP handlers
	h := handlers.NewHandlers(database, cfg, orch)

	// Create router
	router := api.NewRouter(h)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("Shutdown signal received")
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Info().Str("addr", addr).Msg("CEPC Analysis Copilot starting")
	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}