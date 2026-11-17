package service

import (
	"errors"
	"strconv"

	"konsulin-backend/app/model"
	"konsulin-backend/app/repository"
	"konsulin-backend/helper"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type BookingService struct {
	bookings repository.BookingRepository
	slots    repository.SlotRepository
	tx       *helper.TxManager
}

func NewBookingService(b repository.BookingRepository, s repository.SlotRepository, tx *helper.TxManager) *BookingService {
	return &BookingService{
		bookings: b,
		slots:    s,
		tx:       tx,
	}
}

func (s *BookingService) mapError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, repository.ErrSlotFull):
		return helper.Fail(c, fiber.StatusConflict, helper.CodeSlotFull, "kuota slot sudah penuh")
	case errors.Is(err, repository.ErrAlreadyBooked):
		return helper.Fail(c, fiber.StatusConflict, helper.CodeAlreadyBooked, "anda sudah memesan slot ini")
	case errors.Is(err, repository.ErrBookingLimitExceeded):
		return helper.Fail(c, fiber.StatusConflict, helper.CodeBookingLimit, "melebihi batas maksimal 3 booking aktif")
	case errors.Is(err, repository.ErrSlotClosed):
		return helper.Fail(c, fiber.StatusConflict, helper.CodeSlotClosed, "slot sudah ditutup")
	case errors.Is(err, repository.ErrNotBooked):
		return helper.Fail(c, fiber.StatusConflict, helper.CodeNotBooked, "booking tidak ditemukan atau tidak aktif")
	case errors.Is(err, repository.ErrBookingNotFound), errors.Is(err, repository.ErrSlotNotFound):
		return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "data tidak ditemukan")
	case errors.Is(err, repository.ErrUnauthorizedBooking):
		return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "booking bukan milik anda")
	default:
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "terjadi kesalahan server: "+err.Error())
	}
}

func (s *BookingService) Book(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	slotID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID slot tidak valid")
	}

	var req model.BookSlotRequest
	_ = c.BodyParser(&req)

	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	var createdBooking model.Booking
	err = s.tx.WithTx(c.Context(), func(tx helper.DBTX) error {
		b, bookErr := s.bookings.BookSlot(c.Context(), tx, user.UserID, slotID, notes)
		if bookErr != nil {
			return bookErr
		}
		createdBooking = b
		return nil
	})

	if err != nil {
		return s.mapError(c, err)
	}

	return helper.Success(c, fiber.StatusCreated, "booking berhasil dibuat", createdBooking)
}

func (s *BookingService) Cancel(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	slotID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID slot tidak valid")
	}

	var cancelledBooking model.Booking
	isAdmin := user.Role == "admin"

	err = s.tx.WithTx(c.Context(), func(tx helper.DBTX) error {
		var bookingID int64
		var studentID int64

		if isAdmin && c.Query("student_id") != "" {
			studentID, _ = strconv.ParseInt(c.Query("student_id"), 10, 64)
		} else {
			studentID = user.UserID
		}

		findQuery := "SELECT id FROM bookings WHERE slot_id = $1 AND student_id = $2 AND status IN ('booked', 'confirmed')"
		if err := tx.QueryRow(c.Context(), findQuery, slotID, studentID).Scan(&bookingID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ErrNotBooked
			}
			return err
		}

		b, cancelErr := s.bookings.Cancel(c.Context(), tx, bookingID, studentID, isAdmin)
		if cancelErr != nil {
			return cancelErr
		}
		cancelledBooking = b
		return nil
	})

	if err != nil {
		return s.mapError(c, err)
	}

	return helper.Success(c, fiber.StatusOK, "booking berhasil dibatalkan", cancelledBooking)
}

func (s *BookingService) Confirm(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	bookingID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID booking tidak valid")
	}

	var confirmedBooking model.Booking
	err = s.tx.WithTx(c.Context(), func(tx helper.DBTX) error {
		b, confirmErr := s.bookings.Confirm(c.Context(), tx, bookingID, user.UserID)
		if confirmErr != nil {
			return confirmErr
		}
		confirmedBooking = b
		return nil
	})

	if err != nil {
		return s.mapError(c, err)
	}

	return helper.Success(c, fiber.StatusOK, "booking berhasil dikonfirmasi", confirmedBooking)
}

func (s *BookingService) Swap(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	var req model.SwapRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format JSON tidak valid")
	}

	if req.TargetSlotID <= 0 {
		return helper.ErrorResponse(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "validasi gagal", map[string]string{
			"target_slot_id": "target_slot_id wajib diisi",
		})
	}

	var swappedBooking model.Booking
	err := s.tx.WithTx(c.Context(), func(tx helper.DBTX) error {
		bookingID := req.BookingID
		if bookingID == 0 && req.CurrentSlotID > 0 {
			findQuery := "SELECT id FROM bookings WHERE slot_id = $1 AND student_id = $2 AND status IN ('booked', 'confirmed')"
			if err := tx.QueryRow(c.Context(), findQuery, req.CurrentSlotID, user.UserID).Scan(&bookingID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return repository.ErrNotBooked
				}
				return err
			}
		} else if bookingID == 0 {
			return errors.New("booking_id atau current_slot_id wajib diisi")
		}

		b, swapErr := s.bookings.Swap(c.Context(), tx, user.UserID, bookingID, req.TargetSlotID)
		if swapErr != nil {
			return swapErr
		}
		swappedBooking = b
		return nil
	})

	if err != nil {
		return s.mapError(c, err)
	}

	return helper.Success(c, fiber.StatusOK, "slot berhasil dipindahkan", swappedBooking)
}

func (s *BookingService) GetMine(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	list, err := s.bookings.FindByStudent(c.Context(), nil, user.UserID)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil riwayat booking")
	}

	return helper.Success(c, fiber.StatusOK, "riwayat booking berhasil diambil", list)
}

func (s *BookingService) UpdateStatus(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	bookingID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID booking tidak valid")
	}

	var req model.UpdateBookingStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format JSON tidak valid")
	}

	if req.Status != model.StatusAttended && req.Status != model.StatusNoShow {
		return helper.ErrorResponse(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "validasi gagal", map[string]string{
			"status": "status harus berupa 'attended' atau 'no_show'",
		})
	}

	if user.Role != "admin" {
		var lecturerID int64
		err := s.tx.WithTx(c.Context(), func(tx helper.DBTX) error {
			query := `
				SELECT s.lecturer_id
				FROM bookings b
				JOIN slots s ON b.slot_id = s.id
				WHERE b.id = $1
			`
			return tx.QueryRow(c.Context(), query, bookingID).Scan(&lecturerID)
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "booking tidak ditemukan")
			}
			return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal memeriksa izin")
		}
		if lecturerID != user.UserID {
			return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "anda bukan pemilik slot sesi ini")
		}
	}

	updated, err := s.bookings.UpdateStatus(c.Context(), nil, bookingID, req.Status)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "booking tidak ditemukan")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal memperbarui status booking")
	}

	return helper.Success(c, fiber.StatusOK, "status booking berhasil diperbarui", updated)
}
