package main

import "time"

type CurseForgeFile struct {
	Id          int
	FileDate    time.Time
	DownloadUrl string
	ReleaseType int
	IsAvailable bool
	GameVersion []string
}

type CurseForgeProject struct {
	Id         int
	WebsiteUrl string
	GameId     int
}

type Version struct {
	Id          uint
	CurseId     int
	FileId      int
	ModId       string
	Version     string
	Type        string
	ReleaseDate time.Time
}
