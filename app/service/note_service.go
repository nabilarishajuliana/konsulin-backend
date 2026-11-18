package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"konsulin-backend/app/model"
	"konsulin-backend/app/repository"
	"konsulin-backend/helper"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NoteService struct {
	notes repository.NoteRepository
	pool  *pgxpool.Pool
}

func NewNoteService(
	notes repository.NoteRepository,
	pool *pgxpool.Pool,
) *NoteService {
	return &NoteService{
		notes: notes,
		pool:  pool,
	}
}

func (s *NoteService) bookingAccess(
	c *fiber.Ctx,
	bookingID int64,
) (int64, int64, error) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return 0, 0, helper.Fail(
			c,
			fiber.StatusUnauthorized,
			helper.CodeUnauthorized,
			"token tidak valid atau tidak ada",
		)
	}

	var studentID int64
	var lecturerID int64

	err := s.pool.QueryRow(
		c.UserContext(),
		`
			SELECT b.student_id, sl.lecturer_id
			FROM bookings b
			JOIN slots sl ON sl.id = b.slot_id
			WHERE b.id = $1
		`,
		bookingID,
	).Scan(&studentID, &lecturerID)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, helper.Fail(
			c,
			fiber.StatusNotFound,
			helper.CodeNotFound,
			"booking tidak ditemukan",
		)
	}

	if err != nil {
		return 0, 0, helper.Fail(
			c,
			fiber.StatusInternalServerError,
			helper.CodeInternal,
			"gagal memeriksa booking",
		)
	}

	switch user.Role {
	case "mahasiswa":
		if user.UserID != studentID {
			return 0, 0, helper.Fail(
				c,
				fiber.StatusForbidden,
				helper.CodeForbidden,
				"booking bukan milik anda",
			)
		}
	case "dosen":
		if user.UserID != lecturerID {
			return 0, 0, helper.Fail(
				c,
				fiber.StatusForbidden,
				helper.CodeForbidden,
				"anda bukan dosen pemilik slot ini",
			)
		}
	case "admin":
		// Admin boleh membaca catatan.
	default:
		return 0, 0, helper.Fail(
			c,
			fiber.StatusForbidden,
			helper.CodeForbidden,
			"role tidak memiliki akses",
		)
	}

	return studentID, lecturerID, nil
}

func (s *NoteService) Get(c *fiber.Ctx) error {
	bookingID, err := strconv.ParseInt(c.Params("bookingID"), 10, 64)
	if err != nil || bookingID <= 0 {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "booking ID tidak valid")
	}

	if _, _, err := s.bookingAccess(c, bookingID); err != nil {
		return err
	}

	note, err := s.notes.FindByBookingID(c.UserContext(), bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "catatan konsultasi belum tersedia")
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil catatan konsultasi")
	}

	return helper.Success(c, fiber.StatusOK, "catatan konsultasi berhasil diambil", note)
}

func (s *NoteService) Save(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	if user.Role != "mahasiswa" && user.Role != "dosen" {
		return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "role tidak diizinkan menulis catatan")
	}

	bookingID, err := strconv.ParseInt(c.Params("bookingID"), 10, 64)
	if err != nil || bookingID <= 0 {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "booking ID tidak valid")
	}

	studentID, lecturerID, err := s.bookingAccess(c, bookingID)
	if err != nil {
		return err
	}

	var req model.SaveNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format JSON tidak valid")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return helper.ErrorResponse(
			c,
			fiber.StatusUnprocessableEntity,
			helper.CodeValidation,
			"validasi gagal",
			map[string]string{
				"content": "isi catatan wajib diisi",
			},
		)
	}

	note, err := s.notes.Save(
		c.UserContext(),
		model.ConsultationNote{
			BookingID:  bookingID,
			StudentID:  studentID,
			LecturerID: lecturerID,
			Content:    content,
			UpdatedAt:  time.Now().UTC(),
		},
	)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal menyimpan catatan konsultasi")
	}

	return helper.Success(c, fiber.StatusOK, "catatan konsultasi berhasil disimpan", note)
}
