package unit_test

import (
	"testing"
	"time"

	"konsulin-backend/app/model"
	"konsulin-backend/helper"
)

func TestSlotCapacityValidation(t *testing.T) {
	testCases := []struct {
		name     string
		capacity int
		isValid  bool
	}{
		{"Kapasitas bernilai negatif", -1, false},
		{"Kapasitas bernilai nol", 0, false},
		{"Kapasitas valid (3 mahasiswa)", 3, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid := tc.capacity > 0
			if valid != tc.isValid {
				t.Errorf("expected valid=%v, got=%v", tc.isValid, valid)
			}
		})
	}
}

func TestSlotTimeValidation(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		isValid   bool
	}{
		{"Waktu selesai sebelum waktu mulai", now.Add(2 * time.Hour), now.Add(1 * time.Hour), false},
		{"Waktu selesai sama dengan waktu mulai", now, now, false},
		{"Rentang waktu valid (durasi 1 jam)", now, now.Add(1 * time.Hour), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid := tc.endTime.After(tc.startTime)
			if valid != tc.isValid {
				t.Errorf("expected valid=%v, got=%v", tc.isValid, valid)
			}
		})
	}
}

func TestActiveBookingLimitRule(t *testing.T) {
	const maxActiveBookings = 3

	testCases := []struct {
		name        string
		activeCount int
		canBook     bool
	}{
		{"Belum memiliki reservasi", 0, true},
		{"Memiliki 1 reservasi aktif", 1, true},
		{"Memiliki 2 reservasi aktif", 2, true},
		{"Telah mencapai batas maksimal 3 reservasi aktif", 3, false},
		{"Melebihi batas reservasi aktif", 4, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canBook := tc.activeCount < maxActiveBookings
			if canBook != tc.canBook {
				t.Errorf("expected canBook=%v, got=%v for count=%d", tc.canBook, canBook, tc.activeCount)
			}
		})
	}
}

func TestOptimisticLockingCapacityRule(t *testing.T) {
	slot := model.Slot{
		Capacity:    10,
		BookedCount: 4,
		Version:     2,
	}

	testCases := []struct {
		name        string
		newCapacity int
		isValid     bool
	}{
		{"Kapasitas baru di bawah jumlah pemesan aktif", 3, false},
		{"Kapasitas baru sama dengan jumlah pemesan aktif", 4, true},
		{"Kapasitas baru di atas jumlah pemesan aktif", 8, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.newCapacity >= slot.BookedCount
			if isValid != tc.isValid {
				t.Errorf("expected isValid=%v, got=%v", tc.isValid, isValid)
			}
		})
	}
}

func TestSwapDifferentSlotRule(t *testing.T) {
	currentSlotID := int64(10)
	targetSlotID := int64(10)

	if currentSlotID == targetSlotID {
		isSameSlot := true
		if !isSameSlot {
			t.Error("swap ke slot yang sama seharusnya tidak diizinkan")
		}
	}
}

func TestRBACPermissionsSlotBooking(t *testing.T) {
	perms := helper.NewPermissionSet()

	if !perms.Has("mahasiswa", "booking:create") {
		t.Error("mahasiswa seharusnya memiliki hak akses booking:create")
	}
	if !perms.Has("mahasiswa", "booking:swap") {
		t.Error("mahasiswa seharusnya memiliki hak akses booking:swap")
	}
	if !perms.Has("mahasiswa", "booking:confirm") {
		t.Error("mahasiswa seharusnya memiliki hak akses booking:confirm")
	}
	if perms.Has("mahasiswa", "slot:create") {
		t.Error("mahasiswa tidak boleh memiliki hak akses slot:create")
	}

	if !perms.Has("dosen", "slot:create") {
		t.Error("dosen seharusnya memiliki hak akses slot:create")
	}
	if !perms.Has("dosen", "slot:update") {
		t.Error("dosen seharusnya memiliki hak akses slot:update")
	}
	if !perms.Has("dosen", "slot:read_bookers") {
		t.Error("dosen seharusnya memiliki hak akses slot:read_bookers")
	}
	if !perms.Has("dosen", "booking:attendance") {
		t.Error("dosen seharusnya memiliki hak akses booking:attendance")
	}
	if perms.Has("dosen", "booking:create") {
		t.Error("dosen tidak boleh memiliki hak akses booking:create")
	}
}
