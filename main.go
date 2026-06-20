package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/database"
	"academyprometheus/backend/middlewares"
	"academyprometheus/backend/routes"
	"academyprometheus/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	config.SetupLogger(cfg)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := config.ConnectDatabase(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("database connection skipped; app will still serve health checks")
	} else if err := database.AutoMigrate(db); err != nil {
		log.Fatal().Err(err).Msg("database migration failed")
	} else if err := database.Seed(db); err != nil {
		log.Fatal().Err(err).Msg("database seed failed")
	} else if err := services.ReconcileCouponUsageCounts(context.Background(), db); err != nil {
		log.Fatal().Err(err).Msg("coupon usage reconciliation failed")
	} else if err := services.EnsureDefaultAutomationWorkflows(context.Background(), db); err != nil {
		log.Fatal().Err(err).Msg("automation workflow seed failed")
	}
	if db != nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := services.CancelExpiredPendingOrders(ctx, db); err != nil {
						log.Warn().Err(err).Msg("payment expiry sweep failed")
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		go services.StartEmailCampaignWorker(ctx, db)
		go services.StartAutomationWorker(ctx, db)
		go services.StartStorageMigrationWorker(ctx, db, cfg)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middlewares.RequestID())
	router.Use(middlewares.SecurityHeaders())
	router.Use(middlewares.StrictCORS(cfg))
	router.Use(middlewares.RateLimit(cfg.RateLimitPerMinute))

	api := router.Group("/api/v1")
	routes.RegisterPublicRoutes(api, db, cfg)
	routes.RegisterAuthRoutes(api, db, cfg)
	routes.RegisterDashboardRoutes(api, db, cfg)
	routes.RegisterInstructorRoutes(api, db, cfg)
	routes.RegisterAdminRoutes(api, db, cfg)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.AppPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.AppPort).Msg("api server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("api server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("api server shutdown failed")
	}
}
