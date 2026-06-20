package main

import (
	"context"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/database"
	"academyprometheus/backend/services"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	config.SetupLogger(cfg)
	db, err := config.ConnectDatabase(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("connect worker database")
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatal().Err(err).Msg("worker migrate database")
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	log.Info().Msg("worker started")
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if removed, err := services.CleanupExpiredGeneratedObjects(ctx, db, cfg, 100); err != nil {
			log.Error().Err(err).Msg("generated cache cleanup failed")
		} else if removed > 0 {
			log.Info().Int("removed", removed).Msg("generated cache cleanup completed")
		}
		cancel()
		<-ticker.C
	}
}
