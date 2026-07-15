package main

import (
	"encoding/json"
	"strings"

	"github.com/cfwidget/updatejson/curseforge"
)

var JourneyMapModInfoModel curseforge.Project

const JourneyMapModInfo = `{
	"screenshots": [],
	"id": 32274,
	"gameId": 432,
	"name": "JourneyMap",
	"slug": "journeymap",
	"links": {
		"websiteUrl": "https://www.curseforge.com/minecraft/mc-mods/journeymap",
		"wikiUrl": "http://journeymap.info",
		"issuesUrl": "https://github.com/TeamJM/journeymap/issues",
		"sourceUrl": null
	},
	"summary": "",
	"status": 4,
	"downloadCount": 340823843,
	"isFeatured": false,
	"primaryCategoryId": 423,
	"categories": [ ],
	"classId": 6,
	"authors": [ ],
	"logo": {
		"id": 9144,
		"modId": 32274,
		"title": "635421614078544069.png",
		"description": "",
		"thumbnailUrl": "",
		"url": ""
	},
	"mainFileId": 8421931,
	"latestFiles": [ ],
	"latestFilesIndexes": [ ],
	"latestEarlyAccessFilesIndexes": [],
	"dateCreated": "2011-09-19T23:49:04.217Z",
	"dateModified": "2026-07-14T08:31:09.697Z",
	"dateReleased": "2026-07-13T18:05:40.863Z",
	"allowModDistribution": true,
	"gamePopularityRank": 81,
	"isAvailable": true,
	"hasCommentsEnabled": false,
	"thumbsUpCount": 0,
	"serverAffiliation": {
		"isEnabled": true,
		"isDefaultBanner": true,
		"hasDiscount": false,
		"affiliationService": 1,
		"defaultBannerCustomTitle": "JourneyMap",
		"affiliationLink": ""
	},
	"featuredProjectTag": 0
}`

var JourneyMapFile8325605Model curseforge.File

const JourneyMapFile8325605 = `{
	"id": 8325605,
	"gameId": 432,
	"modId": 32274,
	"isAvailable": true,
	"displayName": "journeymap-1.21.11-6.0.0+neoforge",
	"fileName": "journeymap-neoforge-1.21.11-6.0.0.jar",
	"releaseType": 1,
	"fileStatus": 4,
	"hashes": [
		{
			"value": "cdeeceb74a9a3dc0183b8af2951568d337c7ee08",
			"algo": 1
		},
		{
			"value": "515c8b6eaeba573d0b989945910d1eb7",
			"algo": 2
		}
	],
	"fileDate": "2026-06-26T19:55:37.58Z",
	"fileLength": 4160919,
	"downloadCount": 4420,
	"fileSizeOnDisk": 9343503,
	"downloadUrl": "https://edge.forgecdn.net/files/8325/605/journeymap-neoforge-1.21.11-6.0.0.jar",
	"gameVersions": [
		"Client",
		"NeoForge",
		"Server",
		"1.21.11"
	],
	"sortableGameVersions": [
		{
			"gameVersionName": "Client",
			"gameVersionPadded": "0",
			"gameVersion": "",
			"gameVersionReleaseDate": "2022-12-08T00:00:00Z",
			"gameVersionTypeId": 75208
		},
		{
			"gameVersionName": "NeoForge",
			"gameVersionPadded": "0",
			"gameVersion": "",
			"gameVersionReleaseDate": "2023-07-25T00:00:00Z",
			"gameVersionTypeId": 68441
		},
		{
			"gameVersionName": "Server",
			"gameVersionPadded": "0",
			"gameVersion": "",
			"gameVersionReleaseDate": "2022-12-08T00:00:00Z",
			"gameVersionTypeId": 75208
		},
		{
			"gameVersionName": "1.21.11",
			"gameVersionPadded": "0000000001.0000000021.0000000011",
			"gameVersion": "1.21.11",
			"gameVersionReleaseDate": "2025-12-09T16:42:14.27Z",
			"gameVersionTypeId": 77784
		}
	],
	"dependencies": [],
	"alternateFileId": 0,
	"isServerPack": false,
	"fileFingerprint": 2468824285,
	"modules": [ ]
}`

func init() {
	err := json.NewDecoder(strings.NewReader(JourneyMapModInfo)).Decode(&JourneyMapModInfoModel)
	if err != nil {
		panic(err)
	}

	err = json.NewDecoder(strings.NewReader(JourneyMapFile8325605)).Decode(&JourneyMapFile8325605Model)
	if err != nil {
		panic(err)
	}
}
