//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"konsulin-backend/app/model"
	"konsulin-backend/app/repository"
	"konsulin-backend/config"
	"konsulin-backend/database"
	"konsulin-backend/helper"
)

// Uji race condition pemesanan serentak: membuktikan SELECT FOR UPDATE mencegah overbooking
func TestSlotBookingConcurrency(t *testing.T) {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := database.NewPostgres(ctx, cfg.DB)
	if err != nil {
		t.Skipf("Layanan PostgreSQL belum tersedia untuk integration test: %v", err)
	}
	defer pool.Close()

	txManager := helper.NewTxManager(pool)
	slotRepo := repository.NewSlotRepository(pool)
	bookingRepo := repository.NewBookingRepository(pool)

	// 1. Data dosen pengampu
	var lecturerID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO users (username, name, email, password, role)
		VALUES ($1, $2, $3, $4, 'dosen')
		ON CONFLICT (username) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, fmt.Sprintf("lecturer_%d", time.Now().UnixNano()), "Dosen Penguji", fmt.Sprintf("dosen_%d@test.com", time.Now().UnixNano()), "password").Scan(&lecturerID)
	if err != nil {
		t.Fatalf("gagal membuat data user dosen: %v", err)
	}

	// 2. Slot bimbingan dengan kuota 3 mahasiswa
	const slotCapacity = 3
	slotModel := &model.Slot{
		LecturerID: lecturerID,
		Title:      "Konsultasi Skripsi Konkurensi",
		StartTime:  time.Now().Add(2 * time.Hour),
		EndTime:    time.Now().Add(3 * time.Hour),
		Capacity:   slotCapacity,
	}
	slot, err := slotRepo.Create(ctx, nil, slotModel)
	if err != nil {
		t.Fatalf("gagal membuat data slot uji: %v", err)
	}

	// 3. 10 mahasiswa yang akan berebut slot secara bersamaan
	const totalStudents = 10
	studentIDs := make([]int64, totalStudents)
	for i := 0; i < totalStudents; i++ {
		var sID int64
		uName := fmt.Sprintf("mhs_%d_%d", time.Now().UnixNano(), i)
		uEmail := fmt.Sprintf("mhs_%d_%d@test.com", time.Now().UnixNano(), i)
		err = pool.QueryRow(ctx, `
			INSERT INTO users (username, name, email, password, role)
			VALUES ($1, $2, $3, $4, 'mahasiswa')
			RETURNING id
		`, uName, fmt.Sprintf("Mahasiswa %d", i), uEmail, "password").Scan(&sID)
		if err != nil {
			t.Fatalf("gagal membuat akun mahasiswa %d: %v", i, err)
		}
		studentIDs[i] = sID
	}

	// 4. Simulasi perebutan slot dengan 10 goroutine serentak
	var wg sync.WaitGroup
	var successCount int64
	var fullCount int64

	startSignal := make(chan struct{})

	for i := 0; i < totalStudents; i++ {
		wg.Add(1)
		studentID := studentIDs[i]
		go func() {
			defer wg.Done()
			<-startSignal

			err := txManager.WithTx(ctx, func(tx helper.DBTX) error {
				_, bookErr := bookingRepo.BookSlot(ctx, tx, studentID, slot.ID, nil)
				return bookErr
			})

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else if err == repository.ErrSlotFull {
				atomic.AddInt64(&fullCount, 1)
			}
		}()
	}

	close(startSignal)
	wg.Wait()

	// 5. Verifikasi: tepat 3 berhasil dan 7 ditolak (SLOT_FULL)
	if successCount != slotCapacity {
		t.Errorf("diharapkan tepat %d reservasi berhasil, namun didapatkan %d", slotCapacity, successCount)
	}

	expectedFull := int64(totalStudents - slotCapacity)
	if fullCount != expectedFull {
		t.Errorf("diharapkan %d permintaan ditolak karena SLOT_FULL, namun didapatkan %d", expectedFull, fullCount)
	}

	latestSlot, err := slotRepo.FindByID(ctx, nil, slot.ID)
	if err != nil {
		t.Fatalf("gagal mengambil data slot terbaru: %v", err)
	}

	if latestSlot.BookedCount != slotCapacity {
		t.Errorf("inkonsistensi booked_count: diharapkan bernilai %d, namun bernilai %d", slotCapacity, latestSlot.BookedCount)
	}
}
