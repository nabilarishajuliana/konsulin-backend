package model

import "time"

const (
	StatusBooked    = "booked"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
	StatusNoShow    = "no_show"
	StatusAttended  = "attended"
)

type Booking struct {
	ID        int64     `json:"id"`
	SlotID    int64     `json:"slot_id"`
	StudentID int64     `json:"student_id"`
	Status    string    `json:"status"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BookingDetail struct {
	Booking
	SlotTitle       string    `json:"slot_title,omitempty"`
	SlotStartTime   time.Time `json:"slot_start_time,omitempty"`
	SlotEndTime     time.Time `json:"slot_end_time,omitempty"`
	LecturerID      int64     `json:"lecturer_id,omitempty"`
	StudentName     string    `json:"student_name,omitempty"`
	StudentUsername string    `json:"student_username,omitempty"`
}

type BookSlotRequest struct {
	Notes string `json:"notes"`
}

type SwapRequest struct {
	BookingID     int64 `json:"booking_id"`
	CurrentSlotID int64 `json:"current_slot_id"`
	TargetSlotID  int64 `json:"target_slot_id"`
}

type UpdateBookingStatusRequest struct {
	Status string `json:"status"`
}

