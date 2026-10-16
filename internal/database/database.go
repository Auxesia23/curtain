package database

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"

	"curtain/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Connect opens the SQLite database at path and applies schema migrations.
func Connect(path string) (*gorm.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path cannot be empty")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "journal_mode(WAL)")
	dsn := (&url.URL{Scheme: "file", Path: absPath, RawQuery: query.Encode()}).String()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// A single connection avoids SQLite write-lock contention between GORM
	// operations and keeps in-memory transaction behavior predictable.
	sqlDB.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Migrate creates or updates the application tables.
func Migrate(db *gorm.DB) error {
	if db.Migrator().HasTable("project_user_permissions") {
		if err := db.Migrator().DropTable("project_user_permissions"); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.Invite{},
		&models.Project{},
		&models.ProjectUser{},
		&models.Environment{},
		&models.EnvironmentVariable{},
	); err != nil {
		return err
	}
	return ensureAdmin(db)
}

// ensureAdmin promotes the oldest user if an instance has users but no admin.
func ensureAdmin(db *gorm.DB) error {
	var userCount int64
	if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
		return err
	}
	if userCount == 0 {
		return nil
	}

	var adminCount int64
	if err := db.Model(&models.User{}).
		Where("role = ?", models.UserRoleAdmin).
		Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}

	var oldest models.User
	if err := db.Order("id ASC").First(&oldest).Error; err != nil {
		return err
	}
	log.Printf("curtain: no instance admin found; promoting oldest user (%s) to admin", oldest.Email)
	return db.Model(&oldest).Update("role", models.UserRoleAdmin).Error
}
