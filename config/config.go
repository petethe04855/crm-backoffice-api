// Package config loads backoffice-api settings from the environment.
// Variable names follow erp-api-v2 (DB_HOST, DB_USER, DB_PASSWORD, JWT_SECRET)
// so values can be copied from the ERP .env. The CRM keeps its own database
// name (CRM_DB_NAME, default "crm"): DB_NAME is deliberately ignored so the CRM
// never lands in the ERP database. CRM_-prefixed names still win when set.
// See .env.example; secrets are never given defaults.
package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds runtime settings.
type Config struct {
	Port           string
	Environment    string
	AllowedOrigins string

	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	DatabaseURL string
	DBSchema    string
	AutoMigrate bool

	// ERPJWTSecret verifies staff tokens issued by erp-api-v2 (HS256 today).
	ERPJWTSecret      string
	ERPAPIBaseURL     string
	ERPAPIToken       string
	ERPSessionCheck   bool
	ERPSessionTTL     time.Duration
	ERPRequestTimeout time.Duration

	// CRMInternalToken authenticates erp-api-v2 calls to /internal/* (P3).
	CRMInternalToken string

	// WorkerEnabled starts the background job runner (coupon/point expiry).
	WorkerEnabled bool
	// UploadDir is where uploaded images are stored on disk.
	UploadDir string
}

// IsProduction reports whether ENV is production.
func (c *Config) IsProduction() bool { return strings.EqualFold(c.Environment, "production") }

// Load reads .env from the working directory (if present) and the environment.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] no .env file in working directory, using environment variables")
	}
	return fromEnv()
}

func fromEnv() *Config {
	return &Config{
		Port:           getEnv("PORT", "8090"),
		Environment:    getEnv("ENV", "development"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:8092,http://127.0.0.1:8092"),

		DBHost:      firstEnv("localhost", "CRM_DB_HOST", "DB_HOST"),
		DBPort:      firstEnv("5432", "CRM_DB_PORT", "DB_PORT"),
		DBUser:      firstEnv("postgres", "CRM_DB_USER", "DB_USER"),
		DBPassword:  firstEnv("", "CRM_DB_PASSWORD", "DB_PASSWORD"),
		DBName:      getEnv("CRM_DB_NAME", "crm"),
		DBSSLMode:   firstEnv("disable", "CRM_DB_SSLMODE", "DB_SSLMODE"),
		DatabaseURL: getEnv("CRM_DATABASE_URL", ""), // DATABASE_URL is the ERP's database, never reused
		DBSchema:    getEnv("CRM_DB_SCHEMA", "crm"),
		AutoMigrate: getBool("CRM_DB_AUTO_MIGRATE", true),

		ERPJWTSecret:      firstEnv("", "ERP_JWT_SECRET", "JWT_SECRET"),
		ERPAPIBaseURL:     getEnv("ERP_API_BASE_URL", "http://localhost:8084/api/v1"),
		ERPAPIToken:       getEnv("ERP_API_TOKEN", ""),
		ERPSessionCheck:   getBool("ERP_SESSION_CHECK", true),
		ERPSessionTTL:     getDuration("ERP_SESSION_CACHE_TTL", 60*time.Second),
		ERPRequestTimeout: getDuration("ERP_REQUEST_TIMEOUT", 10*time.Second),

		CRMInternalToken: getEnv("CRM_INTERNAL_TOKEN", ""),

		WorkerEnabled: getBool("WORKER_ENABLED", true),
		UploadDir:     getEnv("UPLOAD_DIR", "./uploads"),
	}
}

// placeholder reports values left as in .env.example.
func placeholder(v string) bool {
	return strings.HasPrefix(strings.ToLower(v), "your_")
}

// Validate refuses configurations that would be unsafe or cannot work.
func (c *Config) Validate() error {
	var errs []error
	switch {
	case c.ERPJWTSecret == "":
		errs = append(errs, errors.New("JWT_SECRET is required (copy JWT_SECRET from erp-api-v2/.env)"))
	case placeholder(c.ERPJWTSecret):
		errs = append(errs, errors.New("JWT_SECRET still has the .env.example placeholder; copy the real value from erp-api-v2/.env"))
	case len(c.ERPJWTSecret) < 16:
		errs = append(errs, errors.New("JWT_SECRET must be at least 16 characters"))
	}
	if c.DatabaseURL == "" && placeholder(c.DBPassword) {
		errs = append(errs, errors.New("DB_PASSWORD still has the .env.example placeholder; copy DB_PASSWORD from erp-api-v2/.env"))
	}
	if c.DatabaseURL == "" && c.DBPassword == "" && c.IsProduction() {
		errs = append(errs, errors.New("DB_PASSWORD or CRM_DATABASE_URL is required in production"))
	}
	if c.IsProduction() && !c.ERPSessionCheck {
		errs = append(errs, errors.New("ERP_SESSION_CHECK must stay enabled in production"))
	}
	return errors.Join(errs...)
}

// DSN builds the Postgres connection string for the CRM database.
func (c *Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return c.dsnFor(c.DBName)
}

// MaintenanceDSN connects to the "postgres" database on the same server; used
// only to create the CRM database when it does not exist yet.
func (c *Config) MaintenanceDSN() string {
	if c.DatabaseURL != "" {
		if u, err := url.Parse(c.DatabaseURL); err == nil && u.Scheme != "" {
			u.Path = "/postgres"
			return u.String()
		}
	}
	return c.dsnFor("postgres")
}

func (c *Config) dsnFor(dbName string) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Bangkok",
		c.DBHost, c.DBPort, c.DBUser, quoteDSN(c.DBPassword), dbName, c.DBSSLMode)
}

// quoteDSN quotes a keyword/value DSN value so spaces or quotes in a password work.
func quoteDSN(v string) string {
	if v != "" && !strings.ContainsAny(v, ` '\`) {
		return v
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

// firstEnv returns the first non-empty variable among keys, else fallback.
func firstEnv(fallback string, keys ...string) string {
	for _, k := range keys {
		if v := getEnv(k, ""); v != "" {
			return v
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(getEnv(key, strconv.FormatBool(fallback)))
	if err != nil {
		return fallback
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(getEnv(key, fallback.String()))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
