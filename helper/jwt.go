package helper

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthUser struct {
	UserID   int64
	Username string
	Role     string
}

type JWTManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTLMinutes int) *JWTManager {
	return &JWTManager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: time.Duration(accessTTLMinutes) * time.Minute,
	}
}

func (m *JWTManager) GenerateAccess(u AuthUser) (string, error) {
	claims := jwt.MapClaims{
		"sub":      u.UserID,
		"username": u.Username,
		"role":     u.Role,
		"iss":      m.issuer,
		"exp":      time.Now().Add(m.accessTTL).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (AuthUser, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode tanda tangan tidak valid")
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return AuthUser{}, errors.New("token tidak valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return AuthUser{}, errors.New("klaim token tidak valid")
	}
	sub, _ := claims["sub"].(float64)
	username, _ := claims["username"].(string)
	role, _ := claims["role"].(string)
	return AuthUser{UserID: int64(sub), Username: username, Role: role}, nil
}
