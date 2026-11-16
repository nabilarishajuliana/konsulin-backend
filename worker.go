package main

import (
	"log/slog"

	"konsulin-backend/config"

	"github.com/hibiken/asynq"
)

func startWorker(cfg config.Config, logger *slog.Logger) {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	// Handler diisi Juan, contoh:
	// mux.HandleFunc("booking.created", handler.HandleBookingCreated)

	logger.Info("worker berjalan")
	if err := srv.Run(mux); err != nil {
		logger.Error("worker berhenti", "error", err)
	}
}