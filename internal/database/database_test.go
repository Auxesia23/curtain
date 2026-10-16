package database

import (
	"path/filepath"
	"testing"
)

type legacyProjectPermission struct {
	ID uint `gorm:"primaryKey"`
}

func (legacyProjectPermission) TableName() string { return "project_user_permissions" }

func TestMigrateDropsLegacyProjectPermissionTable(t *testing.T) {
	db, err := Connect(filepath.Join(t.TempDir(), "curtain.db"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Migrator().CreateTable(&legacyProjectPermission{}); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if db.Migrator().HasTable("project_user_permissions") {
		t.Fatal("legacy project permission table was not dropped")
	}
}
