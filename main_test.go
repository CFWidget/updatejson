package main

import (
	"archive/zip"
	"context"
	"os"
	"testing"
	"time"

	"github.com/cfwidget/updatejson/curseforge"
	"github.com/cfwidget/updatejson/database"
	"github.com/cfwidget/updatejson/models"
	"github.com/cfwidget/updatejson/util"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func init() {
	os.Setenv("DB_FILE", "updatejson.db")
	os.Setenv("CORE_KEY_FILE", "core.key")
	os.Setenv("DEBUG", "true")

	database.Initialize()
	var err error
	db, err = database.Get(context.Background())
	if err != nil {
		panic(err)
	}
}

var db *gorm.DB

func Test_areEqual(t *testing.T) {
	type args struct {
		arr1 []string
		arr2 []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Same order",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"abc", "def", "ghi"},
			},
			want: true,
		},
		{
			name: "Different order",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"def", "ghi", "abc"},
			},
			want: true,
		},
		{
			name: "Not same",
			args: args{
				arr1: []string{"abc", "def", "ghi"},
				arr2: []string{"123", "def", "ghi"},
			},
			want: false,
		},
		{
			name: "Arr1 nil",
			args: args{
				arr1: nil,
				arr2: []string{"123", "def", "ghi"},
			},
			want: false,
		},
		{
			name: "Arr2 nil",
			args: args{
				arr1: []string{"123", "def", "ghi"},
				arr2: nil,
			},
			want: false,
		},
		{
			name: "Both nil",
			args: args{
				arr1: nil,
				arr2: nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.AreEqual(tt.args.arr1, tt.args.arr2); got != tt.want {
				t.Errorf("areEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetModLoaderFromJar(t *testing.T) {
	type test struct {
		Name   string
		URL    string
		Loader []string
	}
	tests := []test{
		{
			Name:   "journeymap-forge",
			URL:    "https://edge.forgecdn.net/files/4774/257/journeymap-1.20.1-5.9.15-forge.jar",
			Loader: []string{"forge"},
		},
		{
			Name:   "journeymap-forge-old",
			URL:    "https://edge.forgecdn.net/files/2916/2/journeymap-1.12.2-5.7.1.jar",
			Loader: []string{"forge"},
		},
		{
			Name:   "journeymap-fabric",
			URL:    "https://edge.forgecdn.net/files/3821/710/journeymap-1.19-5.8.5-fabric.jar",
			Loader: []string{"fabric"},
		},
		{
			Name:   "journeymap-neoforge",
			URL:    "https://edge.forgecdn.net/files/4828/101/journeymap-1.20.2-5.9.15-neoforge.jar",
			Loader: []string{"neoforge"},
		},
	}
	for _, v := range tests {
		t.Run(v.Name, func(t *testing.T) {
			ctx := context.Background()

			reader, size, err := downloadFile(v.URL, ctx)
			if !assert.NoError(t, err, "error downloading file") {
				return
			}

			r, err := zip.NewReader(reader, size)
			if !assert.NoError(t, err, "error reading file") {
				return
			}

			var modInfo []*models.Mod
			modInfo = parseJarFile(r, ctx)

			if !assert.NotNil(t, modInfo, "error parsing file") {
				return
			}

			if !assert.Equal(t, v.Loader, modInfo[0].Dependencies[0]) {
				return
			}
		})
	}
}

func Test_UnmarshalTOML(t *testing.T) {
	modInfo := &models.TomlMod{}
	err := toml.Unmarshal([]byte(testTOML), modInfo)
	if !assert.NoError(t, err, "error reading file") {
		return
	}
}

const testTOML = `modLoader="javafml"
# Forge for 1.19 is version 41
loaderVersion="[41,)"
license="All rights reserved"
issueTrackerURL="github.com/MinecraftForge/MinecraftForge/issues"
showAsResourcePack=false

[[mods]]
    modId="examplemod"
    version="1.0.0.0"
    displayName="Example Mod"
    updateJSONURL="minecraftforge.net/versions.json"
    displayURL="minecraftforge.net"
    logoFile="logo.png"
    credits="I'd like to thank my mother and father."
    authors="Author"
    description='''
Lets you craft dirt into diamonds. This is a traditional mod that has existed for eons. It is ancient. The holy Notch created it. Jeb rainbowfied it. Dinnerbone made it upside down. Etc.
    '''
    displayTest="MATCH_VERSION"

[[dependencies.examplemod]]
    modId="forge"
    mandatory=true
    versionRange="[41,)"
    ordering="NONE"
    side="BOTH"

[[dependencies.examplemod]]
    modId="minecraft"
    mandatory=true
    versionRange="[1.19,1.20)"
    ordering="NONE"
    side="BOTH"`

func Test_getModVersion(t *testing.T) {
	tests := []struct {
		name      string
		project   curseforge.Project
		curseFile curseforge.File
		modId     string
		ctx       context.Context
		want      *models.Version
		wantErr   bool
		numRows   int64
	}{
		{
			name:      "journeymap-8325605",
			project:   JourneyMapModInfoModel,
			curseFile: JourneyMapFile8325605Model,
			modId:     "journeymap",
			ctx:       context.Background(),
			want: &models.Version{
				Id:           0,
				CurseId:      32274,
				FileId:       8325605,
				GameVersions: "Client,NeoForge,Server,1.21.11",
				ModId:        "journeymap",
				Version:      "1.21.11-6.0.0",
				Type:         1,
				ReleaseDate:  requireTimeParse("2026-06-26T19:55:37.58Z"),
				Url:          "https://www.curseforge.com/minecraft/mc-mods/journeymap/files/8325605",
				Loader:       "neoforge",
			},
			wantErr: false,
			numRows: 2,
		},
	}

	//remove rows to check behavior
	err := db.Model(&models.Version{}).Where("1=1").Delete("1 = 1").Error
	if err != nil {
		t.Fatal(err)
		return
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := getModVersion(tt.project, tt.curseFile, tt.modId, tt.ctx)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("getModVersion() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("getModVersion() succeeded unexpectedly")
			}

			//since the PK will differ, ignore that from the result
			got.Id = 0
			if !assert.ObjectsAreEqual(tt.want, got) {
				t.Errorf("getModVersion()\n%v, want\n%v", got, tt.want)
			}

			var res int64
			err = db.Model(&models.Version{}).Where(&models.Version{
				CurseId: tt.project.Id,
				FileId:  tt.curseFile.Id,
			}).Count(&res).Error

			if !assert.Equal(t, tt.numRows, res) {
				return
			}

			//now do it as a force
			got, gotErr = getModVersion(tt.project, tt.curseFile, tt.modId, context.WithValue(tt.ctx, "force", true))
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("getModVersion() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("getModVersion() succeeded unexpectedly")
			}

			//since the PK will differ, ignore that from the result
			got.Id = 0
			if !assert.ObjectsAreEqual(tt.want, got) {
				t.Errorf("getModVersion()\n%v, want\n%v", got, tt.want)
			}

			err = db.Model(&models.Version{}).Where(&models.Version{
				CurseId: tt.project.Id,
				FileId:  tt.curseFile.Id,
			}).Count(&res).Error

			if !assert.Equal(t, tt.numRows, res) {
				return
			}
		})
	}
}

func requireTimeParse(d string) time.Time {
	f, e := time.Parse(time.RFC3339Nano, d)
	if e != nil {
		panic(e)
	}
	return f
}
