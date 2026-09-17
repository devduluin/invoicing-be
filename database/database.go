package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"duluin_invoice/config"
)

var DB *gorm.DB

func ConnectDB() error {
	var err error
	DB, err = gorm.Open(postgres.Open(config.AppConfig.DB_URL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err = sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	log.Println("✅ PostgreSQL connected")
	return nil
}

func CloseDB() {
	if DB == nil {
		return
	}
	if sqlDB, err := DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
