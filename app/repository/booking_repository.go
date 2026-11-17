package repository

import (
	"context"
	"encoding/json"
	"errors"

	"konsulin-backend/app/model"
	"konsulin-backend/helper"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBookingNotFound      = errors.New("booking tidak ditemukan")
	ErrSlotFull             = errors.New("kuota slot sudah penuh")
	ErrAlreadyBooked        = errors.New("anda sudah memesan slot ini")
	ErrNotBooked            = errors.New("booking tidak ditemukan atau tidak aktif")
	ErrBookingLimitExceeded = errors.New("melebihi batas maksimal 3 booking aktif")
	ErrSlotClosed           = errors.New("slot sudah ditutup")
	ErrUnauthorizedBooking  = errors.New("booking bukan milik anda")
)

type BookingRepository interface {
	BookSlot(ctx context.Context, q helper.DBTX, studentID, slotID int64, notes *string) (model.Booking, error)
	Cancel(ctx context.Context, q helper.DBTX, bookingID, studentID int64, isAdmin bool) (model.Booking, error)
	Confirm(ctx context.Context, q helper.DBTX, bookingID, studentID int64) (model.Booking, error)
	Swap(ctx context.Context, q helper.DBTX, studentID, bookingID, targetSlotID int64) (model.Booking, error)
	FindByStudent(ctx context.Context, q helper.DBTX, studentID int64) ([]model.BookingDetail, error)
	FindByID(ctx context.Context, q helper.DBTX, id int64) (model.Booking, error)
	FindBySlotID(ctx context.Context, q helper.DBTX, slotID int64) ([]model.BookingDetail, error)
	UpdateStatus(ctx context.Context, q helper.DBTX, bookingID int64, status string) (model.Booking, error)
	ReleaseExpired(ctx context.Context, q helper.DBTX, bookingID int64) (bool, error)
}

type bookingRepository struct {
	pool *pgxpool.Pool
}

func NewBookingRepository(pool *pgxpool.Pool) BookingRepository {
	return &bookingRepository{pool: pool}
}

func (r *bookingRepository) getDBTX(q helper.DBTX) helper.DBTX {
	if q != nil {
		return q
	}
	return r.pool
}

func recordOutbox(ctx context.Context, db helper.DBTX, eventType string, payload any) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx,
		"INSERT INTO outbox (event_type, payload, status, created_at) VALUES ($1, $2, 'pending', now())",
		eventType, bytes)
	return err
}

func (r *bookingRepository) BookSlot(ctx context.Context, q helper.DBTX, studentID, slotID int64, notes *string) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking

	// Kunci baris slot (SELECT FOR UPDATE) untuk mencegah overbooking saat pemesanan serentak
	var capacity, bookedCount int
	var isActive bool
	slotQuery := "SELECT capacity, booked_count, is_active FROM slots WHERE id = $1 FOR UPDATE"
	err := db.QueryRow(ctx, slotQuery, slotID).Scan(&capacity, &bookedCount, &isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return b, ErrSlotNotFound
		}
		return b, err
	}

	if !isActive {
		return b, ErrSlotClosed
	}

	if bookedCount >= capacity {
		return b, ErrSlotFull
	}

	// Batas maksimal 3 booking aktif per mahasiswa
	var activeCount int
	countQuery := "SELECT COUNT(*) FROM bookings WHERE student_id = $1 AND status IN ('booked', 'confirmed')"
	if err := db.QueryRow(ctx, countQuery, studentID).Scan(&activeCount); err != nil {
		return b, err
	}
	if activeCount >= 3 {
		return b, ErrBookingLimitExceeded
	}

	var existingID int64
	checkDupQuery := "SELECT id FROM bookings WHERE slot_id = $1 AND student_id = $2 AND status IN ('booked', 'confirmed')"
	err = db.QueryRow(ctx, checkDupQuery, slotID, studentID).Scan(&existingID)
	if err == nil {
		return b, ErrAlreadyBooked
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return b, err
	}

	insertQuery := `
		INSERT INTO bookings (slot_id, student_id, status, notes, created_at, updated_at)
		VALUES ($1, $2, 'booked', $3, now(), now())
		RETURNING id, slot_id, student_id, status, notes, created_at, updated_at
	`
	err = db.QueryRow(ctx, insertQuery, slotID, studentID, notes).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return b, err
	}

	_, err = db.Exec(ctx, "UPDATE slots SET booked_count = booked_count + 1 WHERE id = $1", slotID)
	if err != nil {
		return b, err
	}

	payload := model.BookingEventPayload{
		BookingID: b.ID,
		SlotID:    b.SlotID,
		StudentID: b.StudentID,
	}
	if err := recordOutbox(ctx, db, model.EventBookingCreated, payload); err != nil {
		return b, err
	}

	return b, nil
}

func (r *bookingRepository) Cancel(ctx context.Context, q helper.DBTX, bookingID, studentID int64, isAdmin bool) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking

	query := "SELECT id, slot_id, student_id, status, notes, created_at, updated_at FROM bookings WHERE id = $1 FOR UPDATE"
	err := db.QueryRow(ctx, query, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return b, ErrNotBooked
		}
		return b, err
	}

	if !isAdmin && b.StudentID != studentID {
		return b, ErrUnauthorizedBooking
	}

	if b.Status != model.StatusBooked && b.Status != model.StatusConfirmed {
		return b, ErrNotBooked
	}

	updateQuery := `
		UPDATE bookings
		SET status = 'cancelled', updated_at = now()
		WHERE id = $1
		RETURNING id, slot_id, student_id, status, notes, created_at, updated_at
	`
	err = db.QueryRow(ctx, updateQuery, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return b, err
	}

	_, err = db.Exec(ctx, "UPDATE slots SET booked_count = booked_count - 1 WHERE id = $1 AND booked_count > 0", b.SlotID)
	if err != nil {
		return b, err
	}

	payload := model.BookingEventPayload{
		BookingID: b.ID,
		SlotID:    b.SlotID,
		StudentID: b.StudentID,
	}
	if err := recordOutbox(ctx, db, model.EventBookingCancelled, payload); err != nil {
		return b, err
	}

	return b, nil
}

func (r *bookingRepository) Confirm(ctx context.Context, q helper.DBTX, bookingID, studentID int64) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking

	query := "SELECT id, slot_id, student_id, status, notes, created_at, updated_at FROM bookings WHERE id = $1 FOR UPDATE"
	err := db.QueryRow(ctx, query, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return b, ErrNotBooked
		}
		return b, err
	}

	if b.StudentID != studentID {
		return b, ErrUnauthorizedBooking
	}

	if b.Status != model.StatusBooked {
		return b, ErrNotBooked
	}

	updateQuery := `
		UPDATE bookings
		SET status = 'confirmed', updated_at = now()
		WHERE id = $1
		RETURNING id, slot_id, student_id, status, notes, created_at, updated_at
	`
	err = db.QueryRow(ctx, updateQuery, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (r *bookingRepository) Swap(ctx context.Context, q helper.DBTX, studentID, bookingID, targetSlotID int64) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking

	query := "SELECT id, slot_id, student_id, status, notes, created_at, updated_at FROM bookings WHERE id = $1 FOR UPDATE"
	err := db.QueryRow(ctx, query, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return b, ErrNotBooked
		}
		return b, err
	}

	if b.StudentID != studentID {
		return b, ErrUnauthorizedBooking
	}

	if b.Status != model.StatusBooked && b.Status != model.StatusConfirmed {
		return b, ErrNotBooked
	}

	oldSlotID := b.SlotID
	if oldSlotID == targetSlotID {
		return b, errors.New("tidak dapat melakukan swap ke slot yang sama")
	}

	// Deadlock prevention: kunci kedua baris slot secara terurut berdasarkan ID
	firstID, secondID := oldSlotID, targetSlotID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	_, err = db.Exec(ctx, "SELECT id FROM slots WHERE id = $1 FOR UPDATE", firstID)
	if err != nil {
		return b, err
	}
	_, err = db.Exec(ctx, "SELECT id FROM slots WHERE id = $1 FOR UPDATE", secondID)
	if err != nil {
		return b, err
	}

	var targetCapacity, targetBooked int
	var targetActive bool
	err = db.QueryRow(ctx, "SELECT capacity, booked_count, is_active FROM slots WHERE id = $1", targetSlotID).
		Scan(&targetCapacity, &targetBooked, &targetActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return b, ErrSlotNotFound
		}
		return b, err
	}

	if !targetActive {
		return b, ErrSlotClosed
	}

	if targetBooked >= targetCapacity {
		return b, ErrSlotFull
	}

	var duplicateCheck int64
	err = db.QueryRow(ctx, "SELECT id FROM bookings WHERE slot_id = $1 AND student_id = $2 AND status IN ('booked', 'confirmed')", targetSlotID, studentID).
		Scan(&duplicateCheck)
	if err == nil {
		return b, ErrAlreadyBooked
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return b, err
	}

	_, err = db.Exec(ctx, "UPDATE slots SET booked_count = booked_count - 1 WHERE id = $1 AND booked_count > 0", oldSlotID)
	if err != nil {
		return b, err
	}

	_, err = db.Exec(ctx, "UPDATE slots SET booked_count = booked_count + 1 WHERE id = $1", targetSlotID)
	if err != nil {
		return b, err
	}

	updateBookingQuery := `
		UPDATE bookings
		SET slot_id = $1, updated_at = now()
		WHERE id = $2
		RETURNING id, slot_id, student_id, status, notes, created_at, updated_at
	`
	err = db.QueryRow(ctx, updateBookingQuery, targetSlotID, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return b, err
	}

	payload := model.BookingEventPayload{
		BookingID: b.ID,
		SlotID:    targetSlotID,
		StudentID: studentID,
		OldSlotID: oldSlotID,
		NewSlotID: targetSlotID,
	}
	if err := recordOutbox(ctx, db, model.EventBookingSwapped, payload); err != nil {
		return b, err
	}

	return b, nil
}

func (r *bookingRepository) FindByStudent(ctx context.Context, q helper.DBTX, studentID int64) ([]model.BookingDetail, error) {
	db := r.getDBTX(q)
	query := `
		SELECT b.id, b.slot_id, b.student_id, b.status, b.notes, b.created_at, b.updated_at,
		       s.title, s.start_time, s.end_time, s.lecturer_id
		FROM bookings b
		JOIN slots s ON b.slot_id = s.id
		WHERE b.student_id = $1
		ORDER BY b.created_at DESC
	`
	rows, err := db.Query(ctx, query, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var details []model.BookingDetail
	for rows.Next() {
		var d model.BookingDetail
		if err := rows.Scan(&d.ID, &d.SlotID, &d.StudentID, &d.Status, &d.Notes, &d.CreatedAt, &d.UpdatedAt,
			&d.SlotTitle, &d.SlotStartTime, &d.SlotEndTime, &d.LecturerID); err != nil {
			return nil, err
		}
		details = append(details, d)
	}
	return details, nil
}

func (r *bookingRepository) FindByID(ctx context.Context, q helper.DBTX, id int64) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking
	query := "SELECT id, slot_id, student_id, status, notes, created_at, updated_at FROM bookings WHERE id = $1"
	err := db.QueryRow(ctx, query, id).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrBookingNotFound
	}
	return b, err
}

func (r *bookingRepository) FindBySlotID(ctx context.Context, q helper.DBTX, slotID int64) ([]model.BookingDetail, error) {
	db := r.getDBTX(q)
	query := `
		SELECT b.id, b.slot_id, b.student_id, b.status, b.notes, b.created_at, b.updated_at,
		       u.name, u.username
		FROM bookings b
		JOIN users u ON b.student_id = u.id
		WHERE b.slot_id = $1
		ORDER BY b.created_at ASC
	`
	rows, err := db.Query(ctx, query, slotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.BookingDetail
	for rows.Next() {
		var d model.BookingDetail
		if err := rows.Scan(&d.ID, &d.SlotID, &d.StudentID, &d.Status, &d.Notes, &d.CreatedAt, &d.UpdatedAt,
			&d.StudentName, &d.StudentUsername); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (r *bookingRepository) UpdateStatus(ctx context.Context, q helper.DBTX, bookingID int64, status string) (model.Booking, error) {
	db := r.getDBTX(q)
	var b model.Booking
	query := `
		UPDATE bookings
		SET status = $1, updated_at = now()
		WHERE id = $2
		RETURNING id, slot_id, student_id, status, notes, created_at, updated_at
	`
	err := db.QueryRow(ctx, query, status, bookingID).
		Scan(&b.ID, &b.SlotID, &b.StudentID, &b.Status, &b.Notes, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrBookingNotFound
	}
	return b, err
}

// ReleaseExpired membatalkan booking kadaluarsa secara idempoten (dipanggil oleh worker Juan)
func (r *bookingRepository) ReleaseExpired(ctx context.Context, q helper.DBTX, bookingID int64) (bool, error) {
	db := r.getDBTX(q)

	var slotID int64
	var status string
	query := "SELECT slot_id, status FROM bookings WHERE id = $1 FOR UPDATE"
	err := db.QueryRow(ctx, query, bookingID).Scan(&slotID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrBookingNotFound
		}
		return false, err
	}

	if status != model.StatusBooked {
		return false, nil
	}

	_, err = db.Exec(ctx, "UPDATE bookings SET status = 'cancelled', updated_at = now() WHERE id = $1", bookingID)
	if err != nil {
		return false, err
	}

	_, err = db.Exec(ctx, "UPDATE slots SET booked_count = booked_count - 1 WHERE id = $1 AND booked_count > 0", slotID)
	if err != nil {
		return false, err
	}

	payload := model.BookingEventPayload{
		BookingID: bookingID,
		SlotID:    slotID,
	}
	_ = recordOutbox(ctx, db, model.EventBookingCancelled, payload)

	return true, nil
}
