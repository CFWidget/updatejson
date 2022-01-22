package main

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
)

var _db *gorm.DB
var locker sync.Once

func Database() (*gorm.DB, error) {
	locker.Do(func() {
		var err error

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_DATABASE"))
		_db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Printf("Error running DB migration: %s", err.Error())
			return
		}

		sqlDB, _ := _db.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)

		if os.Getenv("DB_MODE") != "release" {
			_db = _db.Debug()
			fmt.Printf("Set DB_MODE to 'release' to disable debug database logger \n")
		}

		err = _db.AutoMigrate(&Version{})
		if err != nil {
			log.Printf("Error running DB migration: %s", err.Error())
		}
	})

	return _db, nil
}
