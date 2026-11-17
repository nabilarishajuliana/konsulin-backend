package model

import "time"

type Slot struct {
	ID          int64     `json:"id"`
	LecturerID  int64     `json:"lecturer_id"`
	Title       string    `json:"title"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Capacity    int       `json:"capacity"`
	BookedCount int       `json:"booked_count"`
	Version     int       `json:"version"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateSlotRequest struct {
	Title     string    `json:"title"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Capacity  int       `json:"capacity"`
}

type UpdateCapacityRequest struct {
	Capacity int `json:"capacity"`
	Version  int `json:"version"`
}

