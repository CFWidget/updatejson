package main

import "time"

type Version struct {
	Id          uint `gorm:"primaryKey;autoIncrement"`
	CurseId     int  `gorm:"index"`
	FileId      int  `gorm:"index"`
	ModId       string
	Version     string
	Type        string
	ReleaseDate time.Time
}
