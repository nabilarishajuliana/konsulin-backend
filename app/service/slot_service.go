package service

import (
	"errors"
	"strconv"

	"konsulin-backend/app/model"
	"konsulin-backend/app/repository"
	"konsulin-backend/helper"
	"konsulin-backend/middleware"

	"github.com/gofiber/fiber/v2"
)

type SlotService struct {
	slots    repository.SlotRepository
	bookings repository.BookingRepository
	tx       *helper.TxManager
}

func NewSlotService(s repository.SlotRepository, b repository.BookingRepository, tx *helper.TxManager) *SlotService {
	return &SlotService{
		slots:    s,
		bookings: b,
		tx:       tx,
	}
}

func (s *SlotService) ListAvailable(c *fiber.Ctx) error {
	ctx := c.Context()
	slots, err := s.slots.FindAvailable(ctx, nil)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil daftar slot")
	}
	return helper.Success(c, fiber.StatusOK, "daftar slot berhasil diambil", slots)
}

func (s *SlotService) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID tidak valid")
	}

	ctx := c.Context()
	slot, err := s.slots.FindByID(ctx, nil, id)
	if err != nil {
		if errors.Is(err, repository.ErrSlotNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "slot tidak ditemukan")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil detail slot")
	}

	return helper.Success(c, fiber.StatusOK, "detail slot berhasil diambil", slot)
}

func (s *SlotService) Create(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	var req model.CreateSlotRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format JSON tidak valid")
	}

	errs := make(map[string]string)
	if req.Title == "" {
		errs["title"] = "judul slot wajib diisi"
	}
	if req.Capacity <= 0 {
		errs["capacity"] = "kapasitas harus lebih dari 0"
	}
	if req.StartTime.IsZero() {
		errs["start_time"] = "waktu mulai wajib diisi"
	}
	if req.EndTime.IsZero() {
		errs["end_time"] = "waktu selesai wajib diisi"
	} else if !req.EndTime.After(req.StartTime) {
		errs["end_time"] = "waktu selesai harus setelah waktu mulai"
	}

	if len(errs) > 0 {
		return helper.ErrorResponse(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "validasi gagal", errs)
	}

	slotModel := &model.Slot{
		LecturerID: user.UserID,
		Title:      req.Title,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		Capacity:   req.Capacity,
	}

	ctx := c.Context()
	created, err := s.slots.Create(ctx, nil, slotModel)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal membuat slot konsultasi")
	}

	return helper.Success(c, fiber.StatusCreated, "slot berhasil dibuat", created)
}

func (s *SlotService) GetSlotBookings(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID tidak valid")
	}

	ctx := c.Context()
	slot, err := s.slots.FindByID(ctx, nil, id)
	if err != nil {
		if errors.Is(err, repository.ErrSlotNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "slot tidak ditemukan")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil slot")
	}

	if user.Role != "admin" && slot.LecturerID != user.UserID {
		return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "anda bukan pemilik slot ini")
	}

	list, err := s.bookings.FindBySlotID(ctx, nil, id)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil daftar pemesan")
	}

	return helper.Success(c, fiber.StatusOK, "daftar pemesan slot berhasil diambil", list)
}

func (s *SlotService) UpdateCapacity(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID tidak valid")
	}

	var req model.UpdateCapacityRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format JSON tidak valid")
	}

	if req.Capacity <= 0 {
		return helper.ErrorResponse(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "validasi gagal", map[string]string{
			"capacity": "kapasitas harus lebih dari 0",
		})
	}

	ctx := c.Context()
	slot, err := s.slots.FindByID(ctx, nil, id)
	if err != nil {
		if errors.Is(err, repository.ErrSlotNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "slot tidak ditemukan")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil data slot")
	}

	if user.Role != "admin" && slot.LecturerID != user.UserID {
		return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "anda bukan pemilik slot ini")
	}

	updated, err := s.slots.UpdateCapacity(ctx, nil, id, req.Capacity, req.Version)
	if err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return helper.Fail(c, fiber.StatusConflict, helper.CodeVersionConflict, "versi data telah berubah, silakan muat ulang data terbaru")
		}
		if errors.Is(err, repository.ErrQuotaBelowBooked) {
			return helper.Fail(c, fiber.StatusConflict, helper.CodeQuotaBelowBooked, "kapasitas baru tidak boleh di bawah jumlah booking aktif")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal memperbarui kapasitas slot")
	}

	return helper.Success(c, fiber.StatusOK, "kapasitas slot berhasil diperbarui", updated)
}

func (s *SlotService) Close(c *fiber.Ctx) error {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "token tidak valid atau tidak ada")
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return helper.Fail(c, fiber.StatusBadRequest, helper.CodeValidation, "parameter ID tidak valid")
	}

	ctx := c.Context()
	slot, err := s.slots.FindByID(ctx, nil, id)
	if err != nil {
		if errors.Is(err, repository.ErrSlotNotFound) {
			return helper.Fail(c, fiber.StatusNotFound, helper.CodeNotFound, "slot tidak ditemukan")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil data slot")
	}

	if user.Role != "admin" && slot.LecturerID != user.UserID {
		return helper.Fail(c, fiber.StatusForbidden, helper.CodeForbidden, "anda bukan pemilik slot ini")
	}

	if err := s.slots.Close(ctx, nil, id); err != nil {
		if errors.Is(err, repository.ErrSlotHasActiveBooks) {
			return helper.Fail(c, fiber.StatusConflict, helper.CodeSlotClosed, "tidak dapat menutup slot yang masih memiliki booking aktif")
		}
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal menutup slot")
	}

	return helper.Success(c, fiber.StatusOK, "slot berhasil ditutup", nil)
}
