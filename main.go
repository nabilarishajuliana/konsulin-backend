package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"konsulin-backend/config"
	"konsulin-backend/database"
	"konsulin-backend/helper"
	"konsulin-backend/outbox"
	"konsulin-backend/route"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

func main() {
	role := flag.String("role", "api", "api atau worker")
	flag.Parse()

	cfg := config.Load()
	logger := config.NewLogger(cfg.LogLevel)

	switch *role {
	case "api":
		startAPI(cfg, logger)
	case "worker":
		startWorker(cfg, logger)
	default:
		logger.Error("role tidak dikenal", "role", *role)
		os.Exit(1)
	}
}

func startAPI(cfg config.Config, logger *slog.Logger) {
	ctx := context.Background()

	pool, err := database.NewPostgres(ctx, cfg.DB)
	if err != nil {
		logger.Error("gagal konek postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
		logger.Error("gagal migrasi", "error", err)
		os.Exit(1)
	}

	mongoDB, err := database.NewMongo(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		logger.Error("gagal konek mongo", "error", err)
		os.Exit(1)
	}

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer asynqClient.Close()

	jwt := helper.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTLMinutes)
	perms := helper.NewPermissionSet()
	txManager := helper.NewTxManager(pool)

	app := fiber.New()

	relay := outbox.NewRelay(pool, asynqClient, logger)
	relayCtx, stopRelay := context.WithCancel(ctx)
	go relay.Run(relayCtx)

	route.Register(route.Dependencies{
		App:           app,
		Pool:          pool,
		Mongo:         mongoDB,
		JWT:           jwt,
		Permissions:   perms,
		Tx:            txManager,
		EnableSwagger: cfg.Swagger,
	})

	go func() {
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			logger.Error("server berhenti", "error", err)
		}
	}()
	logger.Info("API berjalan", "port", cfg.AppPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("mematikan server")
	stopRelay()
	_ = app.ShutdownWithContext(ctx)
}