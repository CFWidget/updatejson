package main

import (
	"context"
	"fmt"
	"github.com/cfwidget/updatejson/env"
	mysql "go.elastic.co/apm/module/apmgormv2/v2/driver/mysql"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

var _db *gorm.DB
var locker sync.Once

func Database(ctx context.Context) (*gorm.DB, error) {
	locker.Do(func() {
		var err error

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", env.Get("DB_USER"), env.Get("DB_PASS"), env.Get("DB_HOST"), env.Get("DB_DATABASE"))
		_db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Printf("Error running DB migration: %s", err.Error())
			return
		}

		sqlDB, _ := _db.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)

		if env.Get("DB_MODE") != "release" {
			_db = _db.Debug()
			fmt.Printf("Set DB_MODE to 'release' to disable debug database logger \n")
		}

		err = _db.AutoMigrate(&Version{})
		if err != nil {
			log.Printf("Error running DB migration: %s", err.Error())
		}
	})

	return _db.WithContext(ctx), nil
}
