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

	if err := renameLegacyMCPTables(gdb); err != nil {
		return nil, err
	}

	if err := gdb.AutoMigrate(
		&models.Export{},
		&models.Setting{},
		&models.APIToken{},
		&models.MetricHour{},
		&models.MCPOAuthClient{},
		&models.MCPAuthCode{},
		&models.MCPToken{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := backfillExports(gdb); err != nil {
		return nil, err
	}

	return gdb, nil
}

// renameLegacyMCPTables fixes GORM's bad pluralization of MCPOAuthClient
// (mcpo_auth_clients) to the intended mcp_oauth_clients name used by queries.
func renameLegacyMCPTables(gdb *gorm.DB) error {
	type pair struct{ from, to string }
	for _, p := range []pair{
		{"mcpo_auth_clients", "mcp_oauth_clients"},
	} {
		if !gdb.Migrator().HasTable(p.from) {
			continue
		}
		if gdb.Migrator().HasTable(p.to) {
			// Prefer keeping the correctly named table; drop the legacy empty/dup.
			var n int64
			if err := gdb.Table(p.from).Count(&n).Error; err != nil {
				return fmt.Errorf("count %s: %w", p.from, err)
			}
			if n == 0 {
				if err := gdb.Migrator().DropTable(p.from); err != nil {
					return fmt.Errorf("drop legacy %s: %w", p.from, err)
				}
			}
			continue
		}
		if err := gdb.Exec("ALTER TABLE `" + p.from + "` RENAME TO `" + p.to + "`").Error; err != nil {
			return fmt.Errorf("rename %s -> %s: %w", p.from, p.to, err)
		}
	}
	return nil
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
