package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Relay struct {
	pool   *pgxpool.Pool
	client *asynq.Client
	logger *slog.Logger
}

func NewRelay(pool *pgxpool.Pool, client *asynq.Client, logger *slog.Logger) *Relay {
	return &Relay{pool: pool, client: client, logger: logger}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.publishPending(ctx)
		}
	}
}

func (r *Relay) publishPending(ctx context.Context) {
	rows, err := r.pool.Query(ctx,
		"SELECT id, event_type, payload FROM outbox WHERE status = 'pending' ORDER BY created_at LIMIT 50")
	if err != nil {
		r.logger.Error("gagal membaca outbox", "error", err)
		return
	}

	type event struct {
		id        int64
		eventType string
		payload   []byte
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.id, &e.eventType, &e.payload); err != nil {
			r.logger.Error("gagal scan outbox", "error", err)
			continue
		}
		events = append(events, e)
	}
	rows.Close()

	for _, e := range events {
		task := asynq.NewTask(e.eventType, e.payload)
		_, err := r.client.Enqueue(task,
			asynq.TaskID(fmt.Sprintf("outbox:%d", e.id)),
			asynq.MaxRetry(5))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask) {
			r.logger.Error("gagal enqueue", "id", e.id, "error", err)
			continue
		}
		if _, err := r.pool.Exec(ctx,
			"UPDATE outbox SET status = 'published' WHERE id = $1", e.id); err != nil {
			r.logger.Error("gagal update outbox", "id", e.id, "error", err)
		}
	}
}