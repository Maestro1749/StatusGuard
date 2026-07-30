package main

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/config"
	"StatusGuard/internal/incident"
	"StatusGuard/internal/logger"
	"StatusGuard/internal/monitor"
	"StatusGuard/internal/notification"
	"StatusGuard/internal/ratelimit"
	"StatusGuard/internal/scheduler"
	"StatusGuard/internal/transport"
	"context"
	"database/sql"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad()

	// logger init
	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatal("failed to initialize logger", err)
	}

	// database open
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to open database connection", zap.Error(err))
	}

	if err := db.Ping(); err != nil {
		logger.Fatal("failed to connect to the database", zap.Error(err))
	}
	defer db.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("parse redis url failed", zap.Error(err))
	}

	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("failed to connect to the redis database", zap.Error(err))
	}

	// storages
	monitorRepo := monitor.NewMonitorRepository(db, logger)
	checkerRepo := checker.NewCheckerRepository(db, logger)
	incidentRepo := incident.NewRepository(db, logger)
	redisLimiter := ratelimit.NewRedisLimiter(redisClient, 20*time.Second)

	// notifier
	var notifier notification.Notifier = notification.NewNoopNotifier()

	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
		telegramNotifier, err := notification.NewTelegramNotifier(
			cfg.TelegramBotToken,
			cfg.TelegramChatID,
		)
		if err != nil {
			logger.Error("failed to initialize telegram notifier", zap.Error(err))
		} else {
			notifier = telegramNotifier
			logger.Info("telegram notifier enabled")
		}
	} else {
		logger.Info("telegram notifier disabled, using noop notifier")
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// services
	monitorService := monitor.NewMonitorService(monitorRepo, logger)
	checkerService := checker.NewCheckerService(monitorRepo, checkerRepo, redisLimiter, httpClient, logger)
	incidentService := incident.NewService(incidentRepo, notifier, logger)

	scheduler := scheduler.NewScheduler(
		monitorRepo,
		checkerService,
		incidentService,
		time.Duration(cfg.SchedulerIntervalSeconds)*time.Second,
		cfg.CheckerWorkers,
		logger,
	)

	// handlers
	monitorHandlers := transport.NewMonitorHandler(logger, monitorService)
	checkerHandler := transport.NewCheckerHandler(checkerService, logger)
	incidentHandler := transport.NewIncidentHandler(incidentService, logger)
	healthHandler := transport.NewHealthHandler(logger)

	router := chi.NewRouter()

	// routes
	router.Route("/targets", func(r chi.Router) {
		r.Post("/", monitorHandlers.CreateTarget)
		r.Get("/", monitorHandlers.GetAllTargets)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", monitorHandlers.GetTarget)
			r.Delete("/", monitorHandlers.DeleteTarget)
			r.Patch("/", monitorHandlers.UpdateTarget)

			r.Post("/check", checkerHandler.CheckTarget)
			r.Get("/checks", checkerHandler.GetCheckHistory)
			r.Get("/incidents", incidentHandler.GetAllOpenByTargetID)
		})
	})

	router.Route("/incidents", func(r chi.Router) {
		r.Get("/open", incidentHandler.GetOpen)
	})

	router.Get("/health", healthHandler.Health)

	// graceful shutdown
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.AppPort),
		Handler:           router,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("failed to start http server", zap.Error(err))
			panic(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go scheduler.Start(ctx)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server was forced to shutdown", zap.Error(err))
		panic(err)
	}
}
