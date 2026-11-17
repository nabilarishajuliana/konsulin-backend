package repository

import (
	"context"
	"errors"
	"time"

	"konsulin-backend/app/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrNoteNotFound = errors.New("catatan konsultasi tidak ditemukan")

type NoteRepository interface {
	FindByBookingID(ctx context.Context, bookingID int64) (model.ConsultationNote, error)
	Save(ctx context.Context, note model.ConsultationNote) (model.ConsultationNote, error)
}

type noteRepository struct {
	collection *mongo.Collection
}

func NewNoteRepository(db *mongo.Database) NoteRepository {
	return &noteRepository{
		collection: db.Collection("consultation_notes"),
	}
}

func (r *noteRepository) FindByBookingID(
	ctx context.Context,
	bookingID int64,
) (model.ConsultationNote, error) {
	var note model.ConsultationNote

	err := r.collection.FindOne(
		ctx,
		bson.M{"booking_id": bookingID},
	).Decode(&note)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.ConsultationNote{}, ErrNoteNotFound
	}
	if err != nil {
		return model.ConsultationNote{}, err
	}

	return note, nil
}

func (r *noteRepository) Save(
	ctx context.Context,
	note model.ConsultationNote,
) (model.ConsultationNote, error) {
	now := time.Now().UTC()

	filter := bson.M{"booking_id": note.BookingID}

	update := bson.M{
		"$set": bson.M{
			"student_id":  note.StudentID,
			"lecturer_id": note.LecturerID,
			"content":     note.Content,
			"updated_at":  now,
		},
		"$setOnInsert": bson.M{
			"booking_id": note.BookingID,
			"created_at": now,
		},
	}

	opts := options.Update().SetUpsert(true)

	if _, err := r.collection.UpdateOne(
		ctx,
		filter,
		update,
		opts,
	); err != nil {
		return model.ConsultationNote{}, err
	}

	return r.FindByBookingID(ctx, note.BookingID)
}

var _ NoteRepository = (*noteRepository)(nil)
