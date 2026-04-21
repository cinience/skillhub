package repository

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cinience/skillhub/pkg/config"
	"github.com/cinience/skillhub/pkg/model"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	var db *gorm.DB
	var err error

	switch cfg.Driver {
	case "postgres":
		db, err = gorm.Open(postgres.Open(cfg.URL), gormCfg)
	case "sqlite", "":
		dbPath := cfg.URL
		if dbPath == "" {
			dbPath = "./data/skillhub.db"
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if !cfg.AutoMigrate {
		return db, nil
	}

	// AutoMigrate all models
	if err := db.AutoMigrate(
		&model.User{},
		&model.Skill{},
		&model.SkillVersion{},
		&model.SkillSlugAlias{},
		&model.APIToken{},
		&model.Star{},
		&model.DownloadDedup{},
		&model.AuditLog{},
		&model.SkillDailyStats{},
		&model.ReservedSlug{},
		&model.SkillOwnershipTransfer{},
		&model.Comment{},
		&model.Namespace{},
		&model.NamespaceMember{},
		&model.OAuthIdentity{},
		&model.Notification{},
		&model.Rating{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	// Migrate existing approved skills to public visibility (in transaction)
	if err := db.Transaction(func(tx *gorm.DB) error {
		migrateVisibility(tx)
		seedReservedSlugs(tx)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("post-migration: %w", err)
	}

	return db, nil
}

// migrateVisibility sets existing approved skills with empty visibility to public (one-time migration).
// Only affects skills with empty or NULL visibility — intentionally-private skills are not touched.
func migrateVisibility(db *gorm.DB) {
	result := db.Model(&model.Skill{}).
		Where("moderation_status = ? AND (visibility = '' OR visibility IS NULL)", "approved").
		Where("soft_deleted_at IS NULL").
		Update("visibility", "public")
	if result.RowsAffected > 0 {
		log.Printf("database: migrated %d existing approved skills to public visibility", result.RowsAffected)
	}
}

func seedReservedSlugs(db *gorm.DB) {
	slugs := []string{"admin", "api", "auth", "login", "register", "search", "settings", "system", "help", "about"}
	reason := "system reserved"
	var created int
	for _, slug := range slugs {
		result := db.Where("slug = ?", slug).FirstOrCreate(&model.ReservedSlug{
			Slug:   slug,
			Reason: &reason,
		})
		if result.RowsAffected > 0 {
			created++
		}
	}
	if created > 0 {
		log.Printf("database: seeded %d new reserved slugs", created)
	}
}
