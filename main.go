package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var client = &http.Client{}
var db *gorm.DB
var mcStore *persistence.MemcachedBinaryStore

var ErrUnsupportedGame = errors.New("unsupported game")
var ErrInvalidProjectId = errors.New("invalid project id")

func main() {
	var err error

	//this only works for 1.15+, because that's when the mod.toml in the META-INF was added
	//but because it's hard to do proper version checks, we will just read the files

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_DATABASE"))
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	sqlDB, err := db.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err != nil {
		panic(err)
	}
	if os.Getenv("DB_MODE") != "release" {
		db = db.Debug()
		fmt.Printf("Set DB_MODE to 'release' to disable debug database logger \n")
	}

	err = db.AutoMigrate(&Version{})
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	if os.Getenv("MEMCACHE_SERVERS") != "" {
		servers := os.Getenv("MEMCACHE_SERVERS")
		username := os.Getenv("MEMCACHE_USER")
		password := os.Getenv("MEMCACHE_PASS")
		mcStore = persistence.NewMemcachedBinaryStore(servers, username, password, time.Minute*5)
	}

	if mcStore != nil {
		r.GET("/:projectId/:modId", cache.CachePage(mcStore, persistence.DEFAULT, processRequest))
		r.GET("/:projectId/:modId/expire", expireCache)
	} else {
		r.GET("/:projectId/:modId", processRequest)
	}
	r.StaticFile("/", "home.html")
	r.StaticFile("/app.css", "app.css")
	r.StaticFile("/app.js", "app.js")

	fmt.Printf("Starting web services\n")
	err = r.Run()
	if err != nil {
		panic(err)
	}
}

func processRequest(c *gin.Context) {
	pid := c.Param("projectId")
	modId := c.Param("modId")

	var projectId int
	var err error
	if projectId, err = strconv.Atoi(pid); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	data, err := getUpdateJson(projectId, modId)

	if err == ErrInvalidProjectId {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	} else if err == ErrUnsupportedGame {
		c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if data != nil {
		c.JSON(http.StatusOK, data)
	} else {
		c.Status(http.StatusNotFound)
	}
}

func expireCache(c *gin.Context) {
	key := strings.TrimSuffix(c.Request.RequestURI, "/expire")
	fmt.Printf("Expiring %s\n", key)
	_ = mcStore.Delete(cache.CreateKey(key))
	c.Status(http.StatusAccepted)
}

func getUpdateJson(projectId int, modId string) (*UpdateJson, error) {
	results := make(map[string]CurseForgeFile)

	project, err := getProject(projectId)
	if err != nil {
		return nil, err
	}

	if project.GameId != 432 {
		return nil, ErrUnsupportedGame
	}

	curseforgeFiles, err := getFiles(project.Id)
	if err != nil {
		return nil, err
	}

	for _, file := range curseforgeFiles {
		//because each file can be associated to multiple versions, check each ne
		for _, version := range file.GameVersion {
			if version == "Forge" {
				continue
			}
			key := version + "-latest"
			existing, exists := results[key]
			if !exists {
				results[key] = file
			} else if file.FileDate.After(existing.FileDate) {
				results[key] = file
			}

			if file.ReleaseType == 1 {
				key = version + "-recommended"
				existing, exists = results[key]
				if !exists {
					results[key] = file
				} else if file.FileDate.After(existing.FileDate) {
					results[key] = file
				}
			}
		}
	}

	promos := &UpdateJson{
		Promos:   map[string]string{},
		HomePage: project.WebsiteUrl,
	}

	for k, v := range results {
		version, err := getModVersion(project.Id, v)
		if err != nil {
			return nil, err
		}
		if version.ModId == modId && version.Version != "" {
			promos.Promos[k] = version.Version
		}
	}

	return promos, nil
}

func getProject(projectId int) (CurseForgeProject, error) {
	url := fmt.Sprintf("https://addons-ecs.forgesvc.net/api/v2/addon/%d", projectId)
	fmt.Printf("[GET] %s\n", url)
	response, err := client.Get(url)
	if err != nil {
		return CurseForgeProject{}, err
	}
	defer response.Body.Close()

	var project CurseForgeProject
	if response.StatusCode == 404 {
		return project, ErrInvalidProjectId
	}

	err = json.NewDecoder(response.Body).Decode(&project)
	return project, err
}

func getFiles(projectId int) ([]CurseForgeFile, error) {
	url := fmt.Sprintf("https://addons-ecs.forgesvc.net/api/v2/addon/%d/files", projectId)
	fmt.Printf("[GET] %s\n", url)
	response, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	var curseforgeFiles []CurseForgeFile
	if response.StatusCode == 404 {
		return curseforgeFiles, ErrInvalidProjectId
	}

	err = json.NewDecoder(response.Body).Decode(&curseforgeFiles)
	return curseforgeFiles, err
}

func getModVersion(curseId int, curseFile CurseForgeFile) (*Version, error) {
	version := &Version{
		CurseId: curseId,
		FileId:  curseFile.Id,
	}
	err := db.Where(version).Find(&version).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return version, err
	}

	if err == gorm.ErrRecordNotFound || version.Id == 0 {
		fmt.Printf("[GET]: %s\n", curseFile.DownloadUrl)
		response, err := http.Get(curseFile.DownloadUrl)
		if err != nil {
			return version, nil
		}
		defer response.Body.Close()

		buff := bytes.NewBuffer([]byte{})
		size, err := io.Copy(buff, response.Body)
		if err != nil {
			return version, nil
		}
		response.Body.Close()

		reader := bytes.NewReader(buff.Bytes())
		r, err := zip.NewReader(reader, size)
		if err != nil {
			return version, nil
		}

		var modInfo ModInfo

		for _, file := range r.File {
			if file.Name == "META-INF/mods.toml" {
				fileReader, err := file.Open()
				if err != nil {
					return version, nil
				}
				data, err := io.ReadAll(fileReader)
				if err != nil {
					return version, nil
				}
				err = toml.Unmarshal(data, &modInfo)
				if err != nil {
					return version, nil
				}
				_ = fileReader.Close()
			}
		}

		version.ReleaseDate = curseFile.FileDate
		version.Type = strconv.Itoa(curseFile.ReleaseType)

		if len(modInfo.Mods) > 0 {
			for _, z := range modInfo.Mods {
				version.Id = 0 //resets the id so we can create a new row for this mod id
				version.Version = z.Version
				version.ModId = z.ModId
				err = db.Create(version).Error
				return version, nil
			}
		} else {
			//create with no real data, because it doesn't exist
			err = db.Create(version).Error
			if err != nil {
				return version, nil
			}
		}
	}

	return version, nil
}
