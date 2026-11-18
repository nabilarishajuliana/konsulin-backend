package repository

import (
	"context"
	"errors"
	"time"

	"konsulin-backend/app/model"
	"konsulin-backend/helper"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound        = errors.New("user tidak ditemukan")
	ErrUsernameOrEmailUsed = errors.New("username atau email sudah digunakan")
	ErrRefreshTokenInvalid = errors.New("refresh token tidak valid")
)

type AuthRepository interface {
	FindByUsername(ctx context.Context, username string) (model.User, error)
	FindByID(ctx context.Context, id int64) (model.User, error)
	Create(ctx context.Context, user model.User) (model.User, error)
	StoreRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	RotateRefreshToken(ctx context.Context, q helper.DBTX, oldHash string, newHash string, expiresAt time.Time) (int64, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

type authRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) AuthRepository {
	return &authRepository{
		pool: pool,
	}
}

// FindByUsername mencari user berdasarkan username.
func (r *authRepository) FindByUsername(
	ctx context.Context,
	username string,
) (model.User, error) {
	var u model.User

	err := r.pool.QueryRow(ctx, `
		SELECT
			id,
			username,
			name,
			email,
			password,
			role,
			created_at,
			updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&u.ID,
		&u.Username,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrUserNotFound
	}

	if err != nil {
		return model.User{}, err
	}

	return u, nil
}

// FindByID mencari user berdasarkan ID.
func (r *authRepository) FindByID(
	ctx context.Context,
	id int64,
) (model.User, error) {
	var u model.User

	err := r.pool.QueryRow(ctx, `
		SELECT
			id,
			username,
			name,
			email,
			password,
			role,
			created_at,
			updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&u.ID,
		&u.Username,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrUserNotFound
	}

	if err != nil {
		return model.User{}, err
	}

	return u, nil
}

// Create menyimpan user baru.
func (r *authRepository) Create(
	ctx context.Context,
	u model.User,
) (model.User, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (
			username,
			name,
			email,
			password,
			role
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			username,
			name,
			email,
			password,
			role,
			created_at,
			updated_at
	`,
		u.Username,
		u.Name,
		u.Email,
		u.PasswordHash,
		u.Role,
	).Scan(
		&u.ID,
		&u.Username,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, ErrUsernameOrEmailUsed
		}

		return model.User{}, err
	}

	return u, nil
}

// StoreRefreshToken menyimpan hash refresh token.
func (r *authRepository) StoreRefreshToken(
	ctx context.Context,
	userID int64,
	tokenHash string,
	expiresAt time.Time,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (
			user_id,
			token_hash,
			expires_at,
			revoked
		)
		VALUES ($1, $2, $3, false)
	`, userID, tokenHash, expiresAt)

	return err
}

// RotateRefreshToken mencabut token lama dan menyimpan token baru.
func (r *authRepository) RotateRefreshToken(
	ctx context.Context,
	q helper.DBTX,
	oldHash string,
	newHash string,
	expiresAt time.Time,
) (int64, error) {
	var userID int64

	err := q.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
		  AND revoked = false
		  AND expires_at > now()
		RETURNING user_id
	`, oldHash).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrRefreshTokenInvalid
	}

	if err != nil {
		return 0, err
	}

	_, err = q.Exec(ctx, `
		INSERT INTO refresh_tokens (
			user_id,
			token_hash,
			expires_at,
			revoked
		)
		VALUES ($1, $2, $3, false)
	`, userID, newHash, expiresAt)

	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (r *authRepository) RevokeRefreshToken(
	ctx context.Context,
	tokenHash string,
) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked = true
		WHERE token_hash = $1
		  AND revoked = false
		  AND expires_at > now()
	`, tokenHash)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrRefreshTokenInvalid
	}

	return nil
}

// Memastikan implementasi memenuhi interface.
var _ AuthRepository = (*authRepository)(nil)
var _ helper.DBTX = (*pgxpool.Pool)(nil)
