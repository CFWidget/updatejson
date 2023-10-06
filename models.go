package main

import "time"

type Version struct {
	Id           uint `gorm:"primaryKey;autoIncrement"`
	CurseId      uint `gorm:"index"`
	FileId       uint `gorm:"index"`
	GameVersions string
	ModId        string
	Version      string
	Type         int8 `gorm:"type:tinyint"`
	ReleaseDate  time.Time
	Url          string `gorm:"type:varchar(500)"`
	Loader       string
}
