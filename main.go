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
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const PageSize = 50

var mcStore *persistence.MemcachedBinaryStore

var ErrUnsupportedGame = errors.New("unsupported game")
var ErrInvalidProjectId = errors.New("invalid project id")

func main() {
	var err error

	//this only works for 1.15+, because that's when the mod.toml in the META-INF was added
	//but because it's hard to do proper version checks, we will just read the files

	envCache := os.Getenv("CACHE_TTL")
	cacheTtl := time.Hour
	if envCache != "" {
		cacheTtl, err = time.ParseDuration(envCache)
		if err != nil {
			panic(err)
		}
	}

	r := gin.Default()

	if os.Getenv("MEMCACHE_SERVERS") != "" {
		servers := os.Getenv("MEMCACHE_SERVERS")
		username := os.Getenv("MEMCACHE_USER")
		password := os.Getenv("MEMCACHE_PASS")
		mcStore = persistence.NewMemcachedBinaryStore(servers, username, password, cacheTtl)
	}

	if mcStore != nil {
		r.GET("/:projectId/:modId", cache.CachePage(mcStore, cacheTtl, processRequest))
		r.GET("/:projectId/:modId/expire", expireCache)
	} else {
		r.GET("/:projectId/:modId", processRequest)
	}
	r.StaticFile("/", "home.html")
	r.StaticFile("/app.css", "app.css")
	r.StaticFile("/app.js", "app.js")

	log.Printf("Starting web services\n")
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
	log.Printf("Expiring %s\n", key)
	_ = mcStore.Delete(cache.CreateKey(key))

	c.Status(http.StatusAccepted)
}

func getUpdateJson(projectId int, modId string) (*UpdateJson, error) {
	results := make(map[string]File)

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
		for _, version := range file.GameVersions {
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

	//get each unique file we need to download
	files := make([]File, 0)

	for _, v := range results {
		exists := false
		for _, j := range files {
			if j.Id == v.Id {
				exists = true
				break
			}
		}
		if !exists {
			files = append(files, v)
		}
	}

	versionMap := make(map[int]*Version)
	var wg sync.WaitGroup
	for _, v := range files {
		wg.Add(1)
		go func(file File) {
			defer wg.Done()
			versionInfo, err := getModVersion(project.Id, file)
			if err != nil {
				log.Printf("Error getting mod version from file: %s", err.Error())
			}
			versionMap[file.Id] = versionInfo
		}(v)
	}
	wg.Wait()

	promos := &UpdateJson{
		Promos:   map[string]string{},
		HomePage: project.Links.WebsiteUrl,
	}

	for k, v := range results {
		version, exists := versionMap[v.Id]
		if !exists || version == nil {
			continue
		}
		if version.ModId == modId && version.Version != "" {
			promos.Promos[k] = version.Version
		}
	}

	return promos, nil
}

func getProject(projectId int) (Project, error) {
	url := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", projectId)

	response, err := callCurseForge(url)
	if err != nil {
		return Project{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == 404 {
		return Project{}, ErrInvalidProjectId
	}

	var project ProjectResponse
	err = json.NewDecoder(response.Body).Decode(&project)
	return project.Data, err
}

func getFiles(projectId int) ([]File, error) {
	files := make([]File, 0)
	page := 0

	for {
		response, err := getFilesForPage(projectId, page)
		if err != nil {
			return nil, err
		}

		if len(response.Data) > 0 {
			files = append(files, response.Data...)
		}

		//if we don't have the same number as the page size, we have them all
		if response.Pagination.ResultCount < PageSize {
			break
		}

		page++
	}

	return files, nil
}

func getFilesForPage(projectId, page int) (FileResponse, error) {
	url := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?index=%d&pageSize=%d", projectId, page * PageSize, PageSize)

	response, err := callCurseForge(url)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode == 404 {
		return FileResponse{}, ErrInvalidProjectId
	}

	var curseforgeFiles FileResponse
	err = json.NewDecoder(response.Body).Decode(&curseforgeFiles)
	return curseforgeFiles, err
}

func getModVersion(curseId int, curseFile File) (*Version, error) {
	db, err := Database()
	if err != nil {
		return nil, err
	}

	version := &Version{
		CurseId: curseId,
		FileId:  curseFile.Id,
	}

	err = db.Where(version).Find(&version).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return version, err
	}

	if err == gorm.ErrRecordNotFound || version.Id == 0 {
		/*for _, m := range curseFile.Modules {
		if m.Name != "META-INF" {
			continue
		}*/

		reader, size, err := downloadFile(curseFile.DownloadUrl)
		if err != nil {
			return version, err
		}

		r, err := zip.NewReader(reader, size)
		if err != nil {
			return version, err
		}

		var modInfo ModInfo

		for _, file := range r.File {
			info, exists := checkZipFile(file)
			if exists {
				modInfo = info
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
				return version, err
			}
		}
		//}

		//create with no real data, because it doesn't exist
		err = db.Create(version).Error
		return version, err
	}

	return version, nil
}

func checkZipFile(file *zip.File) (ModInfo, bool) {
	var modInfo ModInfo
	if file.Name == "META-INF/mods.toml" {
		fileReader, err := file.Open()
		if err != nil {
			log.Printf("Error reading mods.toml: %s", err.Error())
			return modInfo, false
		}
		defer fileReader.Close()

		data, err := io.ReadAll(fileReader)
		if err != nil {
			log.Printf("Error reading mods.toml: %s", err.Error())
			return modInfo, false
		}

		err = toml.Unmarshal(data, &modInfo)
		if err != nil {
			log.Printf("Error reading mods.toml: %s", err.Error())
			return modInfo, false
		}
		return modInfo, true
	}
	return modInfo, false
}

func downloadFile(url string) (io.ReaderAt, int64, error) {
	log.Printf("[GET]: %s\n", url)
	response, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}

	defer response.Body.Close()

	buff := bytes.NewBuffer([]byte{})
	size, err := io.Copy(buff, response.Body)
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(buff.Bytes()), size, nil
}
