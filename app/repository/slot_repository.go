package repository

import (
	"context"
	"errors"
	"time"

	"konsulin-backend/app/model"
	"konsulin-backend/helper"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSlotNotFound       = errors.New("slot tidak ditemukan")
	ErrVersionConflict    = errors.New("version conflict")
	ErrQuotaBelowBooked   = errors.New("kuota di bawah jumlah booking")
	ErrSlotHasActiveBooks = errors.New("slot masih memiliki booking aktif")
)

type SlotRepository interface {
	Create(ctx context.Context, q helper.DBTX, s *model.Slot) (model.Slot, error)
	FindByID(ctx context.Context, q helper.DBTX, id int64) (model.Slot, error)
	FindByIDForUpdate(ctx context.Context, q helper.DBTX, id int64) (model.Slot, error)
	FindAvailable(ctx context.Context, q helper.DBTX) ([]model.Slot, error)
	UpdateCapacity(ctx context.Context, q helper.DBTX, id int64, capacity, version int) (model.Slot, error)
	Close(ctx context.Context, q helper.DBTX, id int64) error
	IncrementBookedCount(ctx context.Context, q helper.DBTX, id int64) error
	DecrementBookedCount(ctx context.Context, q helper.DBTX, id int64) error
}

type slotRepository struct {
	pool *pgxpool.Pool
}

func NewSlotRepository(pool *pgxpool.Pool) SlotRepository {
	return &slotRepository{pool: pool}
}

func (r *slotRepository) getDBTX(q helper.DBTX) helper.DBTX {
	if q != nil {
		return q
	}
	return r.pool
}

func (r *slotRepository) Create(ctx context.Context, q helper.DBTX, s *model.Slot) (model.Slot, error) {
	db := r.getDBTX(q)
	query := `
		INSERT INTO slots (lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, 0, 1, true, now())
		RETURNING id, lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at
	`
	var result model.Slot
	err := db.QueryRow(ctx, query, s.LecturerID, s.Title, s.StartTime, s.EndTime, s.Capacity).
		Scan(&result.ID, &result.LecturerID, &result.Title, &result.StartTime, &result.EndTime,
			&result.Capacity, &result.BookedCount, &result.Version, &result.IsActive, &result.CreatedAt)
	return result, err
}

func (r *slotRepository) FindByID(ctx context.Context, q helper.DBTX, id int64) (model.Slot, error) {
	db := r.getDBTX(q)
	query := `
		SELECT id, lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at
		FROM slots
		WHERE id = $1
	`
	var s model.Slot
	err := db.QueryRow(ctx, query, id).
		Scan(&s.ID, &s.LecturerID, &s.Title, &s.StartTime, &s.EndTime,
			&s.Capacity, &s.BookedCount, &s.Version, &s.IsActive, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrSlotNotFound
	}
	return s, err
}

func (r *slotRepository) FindByIDForUpdate(ctx context.Context, q helper.DBTX, id int64) (model.Slot, error) {
	db := r.getDBTX(q)
	query := `
		SELECT id, lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at
		FROM slots
		WHERE id = $1
		FOR UPDATE
	`
	var s model.Slot
	err := db.QueryRow(ctx, query, id).
		Scan(&s.ID, &s.LecturerID, &s.Title, &s.StartTime, &s.EndTime,
			&s.Capacity, &s.BookedCount, &s.Version, &s.IsActive, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrSlotNotFound
	}
	return s, err
}

func (r *slotRepository) FindAvailable(ctx context.Context, q helper.DBTX) ([]model.Slot, error) {
	db := r.getDBTX(q)
	query := `
		SELECT id, lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at
		FROM slots
		WHERE is_active = true AND end_time > $1
		ORDER BY start_time ASC
	`
	rows, err := db.Query(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]model.Slot, 0)
	for rows.Next() {
		var s model.Slot
		if err := rows.Scan(&s.ID, &s.LecturerID, &s.Title, &s.StartTime, &s.EndTime,
			&s.Capacity, &s.BookedCount, &s.Version, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}
	return slots, nil
}

// Optimistic locking menggunakan kolom version
func (r *slotRepository) UpdateCapacity(ctx context.Context, q helper.DBTX, id int64, capacity, version int) (model.Slot, error) {
	db := r.getDBTX(q)
	query := `
		UPDATE slots
		SET capacity = $1, version = version + 1
		WHERE id = $2 AND version = $3 AND capacity >= booked_count
		RETURNING id, lecturer_id, title, start_time, end_time, capacity, booked_count, version, is_active, created_at
	`
	var s model.Slot
	err := db.QueryRow(ctx, query, capacity, id, version).
		Scan(&s.ID, &s.LecturerID, &s.Title, &s.StartTime, &s.EndTime,
			&s.Capacity, &s.BookedCount, &s.Version, &s.IsActive, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			current, fetchErr := r.FindByID(ctx, db, id)
			if fetchErr != nil {
				return s, fetchErr
			}
			if capacity < current.BookedCount {
				return s, ErrQuotaBelowBooked
			}
			if current.Version != version {
				return s, ErrVersionConflict
			}
			return s, ErrVersionConflict
		}
		return s, err
	}
	return s, nil
}

func (r *slotRepository) Close(ctx context.Context, q helper.DBTX, id int64) error {
	db := r.getDBTX(q)
	var activeCount int
	checkQuery := `
		SELECT COUNT(*) FROM bookings
		WHERE slot_id = $1 AND status IN ('booked', 'confirmed')
	`
	if err := db.QueryRow(ctx, checkQuery, id).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return ErrSlotHasActiveBooks
	}

	cmd, err := db.Exec(ctx, "UPDATE slots SET is_active = false WHERE id = $1", id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrSlotNotFound
	}
	return nil
}

func (r *slotRepository) IncrementBookedCount(ctx context.Context, q helper.DBTX, id int64) error {
	db := r.getDBTX(q)
	cmd, err := db.Exec(ctx, "UPDATE slots SET booked_count = booked_count + 1 WHERE id = $1 AND booked_count < capacity", id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("gagal menambah booked_count: kapasitas slot sudah penuh")
	}
	return nil
}

func (r *slotRepository) DecrementBookedCount(ctx context.Context, q helper.DBTX, id int64) error {
	db := r.getDBTX(q)
	cmd, err := db.Exec(ctx, "UPDATE slots SET booked_count = booked_count - 1 WHERE id = $1 AND booked_count > 0", id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("gagal mengurangi booked_count")
	}
	return nil
}
