// Package database holds Postgres helpers for backoffice-api.
package database

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dbNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// IsMissingDatabase reports whether err is Postgres "database does not exist" (3D000).
func IsMissingDatabase(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "3D000") ||
		(strings.Contains(msg, `database "`) && strings.Contains(msg, "does not exist"))
}

// CreateDatabase creates dbName through the maintenance database (like
// erp-api-v2 does on first start). The connecting user needs CREATEDB.
func CreateDatabase(maintenanceDSN, dbName string) error {
	if !dbNamePattern.MatchString(dbName) {
		return fmt.Errorf("invalid database name %q", dbName)
	}
	db, err := gorm.Open(gormpostgres.Open(maintenanceDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return fmt.Errorf("connect maintenance database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)", dbName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(`CREATE DATABASE "` + dbName + `" ENCODING 'UTF8'`).Error; err != nil {
		return errors.Join(fmt.Errorf("create database %q", dbName), err)
	}
	return nil
}
