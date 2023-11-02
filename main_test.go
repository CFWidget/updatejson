package main

import (
	"archive/zip"
	"context"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
			if got := areEqual(tt.args.arr1, tt.args.arr2); got != tt.want {
				t.Errorf("areEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetModLoaderFromJar(t *testing.T) {
	type test struct {
		Name   string
		URL    string
		Loader string
	}
	tests := []test{
		{
			Name:   "journeymap-forge",
			URL:    "https://edge.forgecdn.net/files/4774/257/journeymap-1.20.1-5.9.15-forge.jar",
			Loader: "forge",
		},
		{
			Name:   "journeymap-forge-old",
			URL:    "https://edge.forgecdn.net/files/2916/2/journeymap-1.12.2-5.7.1.jar",
			Loader: "forge",
		},
		{
			Name:   "journeymap-fabric",
			URL:    "https://edge.forgecdn.net/files/3821/710/journeymap-1.19-5.8.5-fabric.jar",
			Loader: "fabric",
		},
		{
			Name:   "journeymap-neoforge",
			URL:    "https://edge.forgecdn.net/files/4828/101/journeymap-1.20.2-5.9.15-neoforge.jar",
			Loader: "neoforge",
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

			var modInfo *ModInfo
			modInfo = parseJarFile(r, ctx)

			if !assert.NotNil(t, modInfo, "error parsing file") {
				return
			}

			if !assert.Equal(t, v.Loader, modInfo.ModLoader) {
				return
			}
		})
	}
}

func Test_UnmarshalTOML(t *testing.T) {
	modInfo := &ModInfo{}
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
