CREATE TABLE IF NOT EXISTS bookings (
    id         BIGSERIAL PRIMARY KEY,
    slot_id    BIGINT NOT NULL REFERENCES slots(id) ON DELETE CASCADE,
    student_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     VARCHAR(20) NOT NULL DEFAULT 'booked' CHECK (status IN ('booked', 'confirmed', 'cancelled', 'no_show', 'attended')),
    notes      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial unique index: mencegah dobel booking aktif di slot yang sama (TDR-13)
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_active_unique ON bookings(slot_id, student_id) WHERE status IN ('booked', 'confirmed');

CREATE INDEX IF NOT EXISTS idx_bookings_student_status ON bookings(student_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_slot ON bookings(slot_id);
