package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var allKeys = []string{
	"CRM_DB_HOST", "DB_HOST", "CRM_DB_PORT", "DB_PORT", "CRM_DB_USER", "DB_USER",
	"CRM_DB_PASSWORD", "DB_PASSWORD", "CRM_DB_NAME", "DB_NAME", "CRM_DB_SSLMODE", "DB_SSLMODE",
	"CRM_DATABASE_URL", "DATABASE_URL", "ERP_JWT_SECRET", "JWT_SECRET", "ENV",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allKeys {
		t.Setenv(k, "")
	}
}

func TestERPStyleNamesAreAccepted(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "erp-pass")
	t.Setenv("DB_NAME", "chawy_erp_v2")
	t.Setenv("DATABASE_URL", "postgres://x@y/chawy_erp_v2")
	t.Setenv("JWT_SECRET", "a-real-secret-123456")

	c := fromEnv()
	require.Equal(t, "db.local", c.DBHost)
	require.Equal(t, "postgres", c.DBUser)
	require.Equal(t, "erp-pass", c.DBPassword)
	require.Equal(t, "crm", c.DBName, "ERP DB_NAME must not move the CRM into the ERP database")
	require.Empty(t, c.DatabaseURL, "ERP DATABASE_URL must not be reused")
	require.Equal(t, "a-real-secret-123456", c.ERPJWTSecret)
	require.NoError(t, c.Validate())
	require.Contains(t, c.DSN(), "dbname=crm")
	require.Contains(t, c.MaintenanceDSN(), "dbname=postgres")
}

func TestCRMNamesWin(t *testing.T) {
	clearEnv(t)
	t.Setenv("DB_PASSWORD", "erp-pass")
	t.Setenv("CRM_DB_PASSWORD", "crm-pass")
	t.Setenv("JWT_SECRET", "a-real-secret-123456")
	t.Setenv("ERP_JWT_SECRET", "crm-specific-secret-1")
	t.Setenv("CRM_DATABASE_URL", "postgres://user:secret@localhost:5432/crm?sslmode=disable")

	c := fromEnv()
	require.Equal(t, "crm-pass", c.DBPassword)
	require.Equal(t, "crm-specific-secret-1", c.ERPJWTSecret)
	require.Equal(t, "postgres://user:secret@localhost:5432/crm?sslmode=disable", c.DatabaseURL)
	require.Equal(t, "postgres://user:secret@localhost:5432/crm?sslmode=disable", c.DSN())
	require.Equal(t, "postgres://user:secret@localhost:5432/postgres?sslmode=disable", c.MaintenanceDSN())
}

func TestValidateRejectsMissingAndPlaceholderValues(t *testing.T) {
	clearEnv(t)
	err := fromEnv().Validate()
	require.ErrorContains(t, err, "JWT_SECRET is required")

	t.Setenv("JWT_SECRET", "your_jwt_secret_key_change_in_production")
	t.Setenv("DB_PASSWORD", "your_password")
	err = fromEnv().Validate()
	require.ErrorContains(t, err, "JWT_SECRET still has the .env.example placeholder")
	require.ErrorContains(t, err, "DB_PASSWORD still has the .env.example placeholder")

	t.Setenv("JWT_SECRET", "short")
	require.ErrorContains(t, fromEnv().Validate(), "at least 16")
}

func TestDSNQuotesPasswordsWithSpaces(t *testing.T) {
	c := &Config{DBHost: "h", DBPort: "5432", DBUser: "u", DBPassword: "a b'c", DBName: "crm", DBSSLMode: "disable"}
	require.True(t, strings.Contains(c.DSN(), `password='a b\'c'`), c.DSN())
	c.DBPassword = "plain123"
	require.Contains(t, c.DSN(), "password=plain123 ")
}
