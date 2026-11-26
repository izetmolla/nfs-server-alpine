package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/nfs"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := gdb.AutoMigrate(&models.Export{}, &models.Setting{}, &models.APIToken{}, &models.MetricHour{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := backfillExports(gdb); err != nil {
		return nil, err
	}

	return gdb, nil
}

func backfillExports(gdb *gorm.DB) error {
	var exports []models.Export
	if err := gdb.Find(&exports).Error; err != nil {
		return err
	}
	for _, e := range exports {
		before := e
		_ = nfs.NormalizeExport(&e)
		if before.AccessMode == "" || before.CommitMode == "" || before.WriteDelay == "" {
			if err := gdb.Save(&e).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func SeedFromEnv(gdb *gorm.DB) error {
	var count int64
	if err := gdb.Model(&models.Export{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	shared := os.Getenv("SHARED_DIRECTORY")
	if shared == "" {
		return nil
	}

	clients := os.Getenv("PERMITTED")
	if clients == "" {
		clients = "*"
	}

	_, readOnlySet := os.LookupEnv("READ_ONLY")
	_, syncSet := os.LookupEnv("SYNC")

	exp := models.DefaultExport()
	exp.Path = shared
	exp.Clients = clients
	if readOnlySet {
		exp.AccessMode = "ro"
		exp.ReadOnly = true
	}
	if syncSet {
		exp.CommitMode = "sync"
		exp.Sync = true
	}
	exp.Fsid = 0
	exp.Enabled = true
	_ = nfs.NormalizeExport(&exp)
	return gdb.Create(&exp).Error
}
