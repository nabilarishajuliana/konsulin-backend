CREATE TABLE IF NOT EXISTS slots (
    id           BIGSERIAL PRIMARY KEY,
    lecturer_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        VARCHAR(100) NOT NULL,
    start_time   TIMESTAMPTZ NOT NULL,
    end_time     TIMESTAMPTZ NOT NULL,
    capacity     INT NOT NULL CHECK (capacity > 0),
    booked_count INT NOT NULL DEFAULT 0 CHECK (booked_count >= 0 AND booked_count <= capacity),
    version      INT NOT NULL DEFAULT 1,
    is_active    BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_slots_lecturer_time ON slots(lecturer_id, start_time);
