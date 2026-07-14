package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cfwidget/updatejson/env"
	"github.com/cfwidget/updatejson/models"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var _db *gorm.DB

func Initialize() {
	var err error

	switch env.GetOr("DB_ENGINE", "sqlite3") {
	case "mysql":
		{
			dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", env.Get("DB_USER"), env.Get("DB_PASS"), env.Get("DB_HOST"), env.Get("DB_DATABASE"))
			_db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		}
	case "sqlite3":
		{
			dsn := env.Get("DB_FILE")
			_db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		}
	default:
		{
			err = errors.New("unsupported DB_ENGINE (one of: sqlite3, mysql), got " + env.Get("DB_ENGINE"))
		}
	}

	if err != nil {
		log.Panicf("Error connecting to DB: %s", err.Error())
		return
	}

	sqlDB, _ := _db.DB()
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if env.Get("DB_MODE") != "release" {
		_db = _db.Debug()
		log.Println("Set DB_MODE to 'release' to disable debug database logger")
	}

	err = _db.AutoMigrate(&models.Version{})
	if err != nil {
		log.Panicf("Error running DB migration: %s", err.Error())
	}

	m := gormigrate.New(_db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID:      "1695655117",
			Migrate: reset,
		},
	})
	err = m.Migrate()
	if err != nil {
		log.Panicf("Error running DB migration: %s", err.Error())
	}
}

func Get(ctx context.Context) (*gorm.DB, error) {
	return _db.WithContext(ctx), nil
}

func reset(db *gorm.DB) error {
	err := db.Exec("TRUNCATE TABLE versions").Error
	if err != nil {
		log.Printf("Error truncating table, using DELETE instea\n%s\n", err)
		err = db.Exec("DELETE FROM versions").Error
	}
	return err
}
