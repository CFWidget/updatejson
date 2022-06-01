package main

import "time"

type Version struct {
	Id           uint   `gorm:"primaryKey;autoIncrement"`
	CurseId      int    `gorm:"index"`
	FileId       int    `gorm:"index"`
	GameVersions string `gorm:"index"`
	ModId        string
	Version      string
	Type         int8 `gorm:"type:tinyint"`
	ReleaseDate  time.Time
	Url          string `gorm:"type:varchar(500)"`
}
