package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"konsulin-backend/app/model"
	"konsulin-backend/app/repository"
	"konsulin-backend/helper"
	"konsulin-backend/middleware"
)

const (
	defaultRole     = "mahasiswa"
	refreshTokenTTL = 30 * 24 * time.Hour
	bcryptCost      = 12
)

type AuthService struct {
	repo       repository.AuthRepository
	jwtManager *helper.JWTManager
	txManager  *helper.TxManager
}

func NewAuthService(
	repo repository.AuthRepository,
	jwtManager *helper.JWTManager,
	txManager *helper.TxManager,
) *AuthService {
	return &AuthService{
		repo:       repo,
		jwtManager: jwtManager,
		txManager:  txManager,
	}
}

// membuat akun pengguna baru.
func (s *AuthService) Register(c *fiber.Ctx) error {
	var req model.RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format request tidak valid")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" ||
		req.Name == "" ||
		req.Email == "" ||
		req.Password == "" {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "username, nama, email, dan password wajib diisi")
	}

	if len(req.Password) < 8 {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "password minimal 8 karakter")
	}

	if len([]byte(req.Password)) > 72 {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "password maksimal 72 byte")
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcryptCost,
	)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal memproses password")
	}

	// tidak dapat mendaftarkan dirinya sebagai admin.
	user := model.User{
		Username:     req.Username,
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
		Role:         defaultRole,
	}

	createdUser, err := s.repo.Create(c.UserContext(), user)
	if err != nil {
		if errors.Is(err, repository.ErrUsernameOrEmailUsed) {
			return helper.Fail(
				c,
				fiber.StatusConflict,
				"CONFLICT",
				"username atau email sudah digunakan",
			)
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal membuat akun")
	}

	return helper.Success(c, fiber.StatusCreated, "registrasi berhasil", toMeResponse(createdUser))
}

func (s *AuthService) Login(c *fiber.Ctx) error {
	var req model.LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format request tidak valid")
	}

	req.Username = strings.TrimSpace(req.Username)

	if req.Username == "" || req.Password == "" {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "username dan password wajib diisi")
	}

	user, err := s.repo.FindByUsername(
		c.UserContext(),
		req.Username,
	)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "username atau password salah")
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil data pengguna")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "username atau password salah")
	}

	tokens, err := s.createTokens(c, user)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal membuat token")
	}

	return helper.Success(c, fiber.StatusOK, "login berhasil", tokens)
}

// Me mengambil data pengguna yang sedang login.
func (s *AuthService) Me(c *fiber.Ctx) error {
	authUser, ok := currentAuthUser(c)
	if !ok {
		return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "belum login")
	}

	user, err := s.repo.FindByID(
		c.UserContext(),
		authUser.UserID,
	)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "pengguna tidak ditemukan")
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal mengambil data pengguna")
	}

	return helper.Success(c, fiber.StatusOK, "data pengguna berhasil diambil", toMeResponse(user))
}

func (s *AuthService) Refresh(c *fiber.Ctx) error {
	var req model.RefreshRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format request tidak valid")
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "refresh_token wajib diisi")
	}

	oldHash := hashToken(req.RefreshToken)

	newRawToken, newHash, err := generateRefreshToken()
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal membuat refresh token")
	}

	ctx := c.UserContext()
	expiresAt := time.Now().Add(refreshTokenTTL)

	var response model.AuthTokensResponse

	err = s.txManager.WithTx(ctx, func(tx helper.DBTX) error {
		userID, err := s.repo.RotateRefreshToken(
			ctx,
			tx,
			oldHash,
			newHash,
			expiresAt,
		)
		if err != nil {
			return err
		}

		user, err := s.repo.FindByID(ctx, userID)
		if err != nil {
			return err
		}

		accessToken, err := s.jwtManager.GenerateAccess(
			helper.AuthUser{
				UserID:   user.ID,
				Username: user.Username,
				Role:     user.Role,
			},
		)
		if err != nil {
			return err
		}

		response = model.AuthTokensResponse{
			User:         toMeResponse(user),
			AccessToken:  accessToken,
			RefreshToken: newRawToken,
			TokenType:    "Bearer",
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenInvalid) {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "refresh token tidak valid atau sudah digunakan")
		}

		if errors.Is(err, repository.ErrUserNotFound) {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "pengguna tidak ditemukan")
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal memperbarui token")
	}

	return helper.Success(c, fiber.StatusOK, "token berhasil diperbarui", response)
}

func (s *AuthService) Logout(c *fiber.Ctx) error {
	var req model.RefreshRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "format request tidak valid")
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		return helper.Fail(c, fiber.StatusUnprocessableEntity, helper.CodeValidation, "refresh_token wajib diisi")
	}

	err := s.repo.RevokeRefreshToken(
		c.UserContext(),
		hashToken(req.RefreshToken),
	)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenInvalid) {
			return helper.Fail(c, fiber.StatusUnauthorized, helper.CodeUnauthorized, "refresh token tidak valid")
		}

		return helper.Fail(c, fiber.StatusInternalServerError, helper.CodeInternal, "gagal logout")
	}

	return helper.Success(c, fiber.StatusOK, "logout berhasil", nil)
}

func (s *AuthService) createTokens(
	c *fiber.Ctx,
	user model.User,
) (*model.AuthTokensResponse, error) {
	authUser := helper.AuthUser{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}

	accessToken, err := s.jwtManager.GenerateAccess(authUser)
	if err != nil {
		return nil, err
	}

	rawToken, tokenHash, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	err = s.repo.StoreRefreshToken(
		c.UserContext(),
		user.ID,
		tokenHash,
		time.Now().Add(refreshTokenTTL),
	)
	if err != nil {
		return nil, err
	}

	return &model.AuthTokensResponse{
		User:         toMeResponse(user),
		AccessToken:  accessToken,
		RefreshToken: rawToken,
		TokenType:    "Bearer",
	}, nil
}

func generateRefreshToken() (string, string, error) {
	bytes := make([]byte, 32)

	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	rawToken := base64.RawURLEncoding.EncodeToString(bytes)
	return rawToken, hashToken(rawToken), nil
}

// hashToken agar token asli tidak disimpan di database.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func toMeResponse(user model.User) model.MeResponse {
	permissions := helper.NewPermissionSet().PermissionsOf(user.Role)

	return model.MeResponse{
		ID:          user.ID,
		Username:    user.Username,
		Name:        user.Name,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: permissions,
	}
}

func currentAuthUser(c *fiber.Ctx) (helper.AuthUser, bool) {
	return middleware.CurrentUser(c)
}
