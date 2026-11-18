package model

const (
	EventBookingCreated   = "booking.created"
	EventBookingCancelled = "booking.cancelled"
	EventBookingSwapped   = "booking.swapped"
)

type BookingEventPayload struct {
	BookingID int64 `json:"booking_id"`
	SlotID    int64 `json:"slot_id"`
	StudentID int64 `json:"student_id"`
	OldSlotID int64 `json:"old_slot_id,omitempty"`
	NewSlotID int64 `json:"new_slot_id,omitempty"`
}
