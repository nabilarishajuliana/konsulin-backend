package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort   string
	LogLevel  string
	DB        DBConfig
	JWT       JWTConfig
	MongoURI  string
	MongoDB   string
	RedisAddr string
	Swagger   bool
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	MaxConns int32
}

type JWTConfig struct {
	Secret           string
	Issuer           string
	AccessTTLMinutes int
	RefreshTTLDays   int
}

func Load() Config {
	_ = godotenv.Load()
	return Config{
		AppPort:  getEnv("APP_PORT", "3000"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "konsulin"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxConns: int32(getEnvInt("DB_MAX_CONNS", 10)),
		},
		JWT: JWTConfig{
			Secret:           getEnv("JWT_SECRET", ""),
			Issuer:           getEnv("JWT_ISSUER", "konsulin"),
			AccessTTLMinutes: getEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
			RefreshTTLDays:   getEnvInt("JWT_REFRESH_TTL_DAYS", 7),
		},
		MongoURI:  getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:   getEnv("MONGO_DATABASE", "konsulin"),
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
		Swagger:   getEnv("SWAGGER_ENABLED", "false") == "true",
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}