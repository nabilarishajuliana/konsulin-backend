package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"konsulin-backend/app/model"
	"konsulin-backend/config"
)

func startWorker(cfg config.Config, logger *slog.Logger) {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: 10},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(model.EventBookingCreated, func(
		ctx context.Context,
		task *asynq.Task,
	) error {
		var payload model.BookingEventPayload

		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("gagal membaca payload booking.created: %w", err)
		}

		if payload.BookingID <= 0 ||
			payload.SlotID <= 0 ||
			payload.StudentID <= 0 {
			return fmt.Errorf("payload booking.created tidak valid")
		}

		logger.Info(
			"event booking dibuat",
			"booking_id", payload.BookingID,
			"slot_id", payload.SlotID,
			"student_id", payload.StudentID,
		)

		return nil
	})

	// Memproses event booking.cancelled
	mux.HandleFunc(model.EventBookingCancelled, func(
		ctx context.Context,
		task *asynq.Task,
	) error {
		var payload model.BookingEventPayload

		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("gagal membaca payload booking.cancelled: %w", err)
		}

		if payload.BookingID <= 0 {
			return fmt.Errorf("payload booking.cancelled tidak valid")
		}

		logger.Info(
			"event booking dibatalkan",
			"booking_id", payload.BookingID,
			"slot_id", payload.SlotID,
			"student_id", payload.StudentID,
		)

		return nil
	})

	// Memproses event booking.swapped
	mux.HandleFunc(model.EventBookingSwapped, func(
		ctx context.Context,
		task *asynq.Task,
	) error {
		var payload model.BookingEventPayload

		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("gagal membaca payload booking.swapped: %w", err)
		}

		if payload.BookingID <= 0 ||
			payload.StudentID <= 0 ||
			payload.OldSlotID <= 0 ||
			payload.NewSlotID <= 0 {
			return fmt.Errorf("payload booking.swapped tidak valid")
		}

		logger.Info(
			"event booking dipindahkan",
			"booking_id", payload.BookingID,
			"student_id", payload.StudentID,
			"old_slot_id", payload.OldSlotID,
			"new_slot_id", payload.NewSlotID,
		)

		return nil
	})

	logger.Info(
		"worker berjalan",
		"redis", cfg.RedisAddr,
		"concurrency", 10,
	)

	if err := srv.Run(mux); err != nil {
		logger.Error("worker berhenti", "error", err)
	}
}
