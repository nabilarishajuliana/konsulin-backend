package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ConsultationNote struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	BookingID  int64              `json:"booking_id" bson:"booking_id"`
	StudentID  int64              `json:"student_id" bson:"student_id"`
	LecturerID int64              `json:"lecturer_id" bson:"lecturer_id"`
	Content    string             `json:"content" bson:"content"`
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at" bson:"updated_at"`
}

type SaveNoteRequest struct {
	Content string `json:"content"`
}
