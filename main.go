package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml"
	"go.elastic.co/apm/module/apmgin/v2"
	"go.elastic.co/apm/v2"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const PageSize = 50

var ErrUnsupportedGame = errors.New("unsupported game")
var ErrInvalidProjectId = errors.New("invalid project id")

var invalidVersions = []string{"Forge", "Fabric", "Quilt", "Rift"}

func main() {
	var err error

	db, err := Database(context.Background())
	if err != nil {
		panic(err)
	}

	err = db.AutoMigrate(&Version{})
	if err != nil {
		panic(err)
	}

	//this only works for 1.15+, because that's when the mod.toml in the META-INF was added
	//but because it's hard to do proper version checks, we will just read the files
	_ = apm.DefaultTracer()

	r := gin.Default()
	r.Use(apmgin.Middleware(r))

	r.GET("/:projectId/:modId", setTransaction, readFromCache, processRequest)
	r.GET("/:projectId/:modId/references", setTransaction, readFromCache, getReferences)
	r.GET("/:projectId/:modId/expire", setTransaction, expireCache)

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
	loader := c.Query("ml")
	if loader == "" {
		loader = c.Query("loader")
		if loader == "" {
			loader = "forge"
		}
	}

	loader = strings.ToLower(loader)

	var projectId int
	var err error
	if projectId, err = strconv.Atoi(pid); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	data, err := getUpdateJson(projectId, modId, loader, c.Request.Context())

	if err == ErrInvalidProjectId || err == ErrUnsupportedGame {
		d := map[string]string{"error": err.Error()}
		SetInCache(c.Request.URL.RequestURI(), http.StatusOK, d)
		c.JSON(http.StatusBadRequest, d)
	} else if err != nil {
		log.Printf("Error: %s", err.Error())
		d := map[string]string{"error": err.Error()}
		cacheExpireTime := SetInCache(c.Request.URL.RequestURI(), http.StatusInternalServerError, d)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusInternalServerError)
		return
	}

	if data != nil {
		cacheExpireTime := SetInCache(c.Request.URL.RequestURI(), http.StatusOK, *data)
		cacheHeaders(c, cacheExpireTime)
		c.JSON(http.StatusOK, data)
	} else {
		cacheExpireTime := SetInCache(c.Request.URL.RequestURI(), http.StatusNotFound, nil)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusNotFound)
	}
}

func expireCache(c *gin.Context) {
	key := strings.TrimSuffix(c.Request.RequestURI, "/expire")
	RemoveFromCache(key)

	key = strings.TrimSuffix(c.Request.RequestURI, "/expire") + "/references"
	RemoveFromCache(key)

	c.Status(http.StatusAccepted)
}

func getReferences(c *gin.Context) {
	pid := c.Param("projectId")
	modId := c.Param("modId")
	loader := c.Query("ml")
	if loader == "" {
		loader = "forge"
	}

	loader = strings.ToLower(loader)

	var projectId int
	var err error
	if projectId, err = strconv.Atoi(pid); err != nil {
		SetInCache(c.Request.URL.RequestURI(), http.StatusNotFound, nil)
		c.Status(http.StatusNotFound)
		return
	}

	db, err := Database(c.Request.Context())
	if err != nil {
		log.Printf("Error: %s", err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	files := make([]Version, 0)
	err = db.Model(&Version{}).Where("curse_id = ? AND mod_id = ?", projectId, modId).Find(&files).Error
	if err != nil {
		log.Printf("Error: %s", err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	results := make(map[string]Version)
	for _, file := range files {
		versions := strings.Split(file.GameVersions, ",")
		if !contains(loader, versions) {
			continue
		}

		for _, version := range versions {
			if contains(version, invalidVersions) {
				continue
			}
			key := version + "-latest"
			existing, exists := results[key]
			if !exists {
				results[key] = file
			} else if file.ReleaseDate.After(existing.ReleaseDate) {
				results[key] = file
			}

			if file.Type == 1 {
				key = version + "-recommended"
				existing, exists = results[key]
				if !exists {
					results[key] = file
				} else if file.ReleaseDate.After(existing.ReleaseDate) {
					results[key] = file
				}
			}
		}
	}

	references := References{}
	for k, v := range results {
		references[k] = v.Url
	}

	cacheExpireTime := SetInCache(c.Request.URL.RequestURI(), http.StatusOK, references)
	cacheHeaders(c, cacheExpireTime)
	c.JSON(http.StatusOK, references)
}

func getUpdateJson(projectId int, modId string, loader string, ctx context.Context) (*UpdateJson, error) {
	results := make(map[string]File)

	project, err := getProject(projectId, ctx)
	if err != nil {
		return nil, err
	}

	if project.GameId != 432 {
		return nil, ErrUnsupportedGame
	}

	curseforgeFiles, err := getFiles(project.Id, ctx)
	if err != nil {
		return nil, err
	}

	filteredFiles := make([]File, 0)

	for _, file := range curseforgeFiles {
		if contains(loader, file.GameVersions) {
			filteredFiles = append(filteredFiles, file)
		}
	}

	for _, file := range filteredFiles {
		//because each file can be associated to multiple versions, check each one
		for _, version := range file.GameVersions {
			if contains(version, invalidVersions) {
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
	var writer sync.Mutex
	for _, v := range files {
		wg.Add(1)
		go func(file File) {
			defer wg.Done()
			versionInfo, err := getModVersion(project, file, modId, ctx)
			if err != nil {
				log.Printf("Error getting mod version from file: %s", err.Error())
			}
			writer.Lock()
			defer writer.Unlock()
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

func getProject(projectId int, ctx context.Context) (Project, error) {
	url := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", projectId)

	response, err := callCurseForge(url, ctx)
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

func getFiles(projectId int, ctx context.Context) ([]File, error) {
	files := make([]File, 0)
	page := 0

	for {
		response, err := getFilesForPage(projectId, page, ctx)
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

func getFilesForPage(projectId, page int, ctx context.Context) (FileResponse, error) {
	url := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?index=%d&pageSize=%d", projectId, page*PageSize, PageSize)

	response, err := callCurseForge(url, ctx)
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

func getModVersion(project Project, curseFile File, modId string, ctx context.Context) (*Version, error) {
	db, err := Database(ctx)
	if err != nil {
		return nil, err
	}

	version := &Version{
		CurseId: project.Id,
		FileId:  curseFile.Id,
	}

	err = db.Where(version).Find(&version).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return version, err
	}

	if err == gorm.ErrRecordNotFound || version.Id == 0 {
		reader, size, err := downloadFile(curseFile.DownloadUrl, ctx)
		if err != nil {
			return version, err
		}

		r, err := zip.NewReader(reader, size)
		if err != nil {
			return version, err
		}

		var manifestVersion string
		manifestVersion, err = getManifestVersion(r)
		if err != nil {
			//ignore errors for now
		}

		var modInfo ModInfo
		for _, file := range r.File {
			info, exists := checkZipFile(file, ctx)
			if exists {
				modInfo = info
			}
		}

		//update info if manifest has a version
		if manifestVersion != "" {
			for k, v := range modInfo.Mods {
				if v.Version == "${file.jarVersion}" {
					modInfo.Mods[k] = Mod{
						ModId:   v.ModId,
						Version: manifestVersion,
					}
				}
			}
		}

		version.ReleaseDate = curseFile.FileDate
		version.Type = curseFile.ReleaseType

		if len(modInfo.Mods) > 0 {
			var matchingVersion *Version
			for _, z := range modInfo.Mods {
				version.Id = 0 //resets the id so we can create a new row for this mod id
				version.Version = z.Version
				version.ModId = z.ModId
				version.Url = fmt.Sprintf("%s/files/%d", project.Links.WebsiteUrl, curseFile.Id)
				version.GameVersions = strings.Join(curseFile.GameVersions, ",")
				err = db.Create(version).Error
				if err != nil {
					return version, err
				}
				if version.ModId == modId {
					matchingVersion = &Version{
						Id:           version.Id,
						CurseId:      version.CurseId,
						FileId:       version.FileId,
						GameVersions: version.GameVersions,
						ModId:        version.ModId,
						Version:      version.Version,
						Type:         version.Type,
						ReleaseDate:  version.ReleaseDate,
						Url:          version.Url,
					}
				}
			}

			return matchingVersion, err
		}

		//create with no real data, because it doesn't exist
		err = db.Create(version).Error
		return version, err
	}

	currentVersions := strings.Split(version.GameVersions, ",")
	if !areEqual(currentVersions, curseFile.GameVersions) {
		version.GameVersions = strings.Join(curseFile.GameVersions, ",")
		err = db.Save(version).Error
	}

	if version.Type != curseFile.ReleaseType {
		version.Type = curseFile.ReleaseType
		err = db.Save(version).Error
	}

	return version, err
}

func checkZipFile(file *zip.File, ctx context.Context) (ModInfo, bool) {
	var modInfo ModInfo
	if file.Name == "META-INF/mods.toml" {
		data, err := readZipEntry(file)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo, false
		}

		err = toml.Unmarshal(data, &modInfo)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo, false
		}

		return modInfo, true
	}

	if file.Name == "fabric.mod.json" || file.Name == "quilt.mod.json" {
		data, err := readZipEntry(file)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo, false
		}

		var mod Mod
		err = json.Unmarshal(data, &mod)
		modInfo = ModInfo{Mods: []Mod{mod}}
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo, false
		}
		return modInfo, true
	}

	return modInfo, false
}

func getManifestVersion(reader *zip.Reader) (string, error) {
	for _, file := range reader.File {
		if file.Name == "META-INF/MANIFEST.MF" {
			//pull out Implementation-Version from the manifest, in case we need it for mods.toml
			data, err := readZipEntry(file)
			if err != nil {
				return "", err
			}

			metadata := readManifest(data)
			return metadata["Implementation-Version"], nil
		}
	}

	return "", nil
}

func downloadFile(url string, ctx context.Context) (io.ReaderAt, int64, error) {
	response, err := download(url, ctx)
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

func areEqual(arr1, arr2 []string) bool {
	if arr1 == nil && arr2 == nil {
		return true
	}

	if arr1 == nil && arr2 != nil {
		return false
	}

	if arr2 == nil && arr1 != nil {
		return false
	}

	for _, a := range arr1 {
		exists := false
		for _, b := range arr2 {
			if a == b {
				exists = true
				break
			}
		}
		if !exists {
			return false
		}
	}

	for _, a := range arr2 {
		exists := false
		for _, b := range arr1 {
			if a == b {
				exists = true
				break
			}
		}
		if !exists {
			return false
		}
	}

	return true
}

func contains(needle string, haystack []string) bool {
	needle = strings.ToLower(needle)
	for _, v := range haystack {
		if strings.ToLower(v) == needle {
			return true
		}
	}

	return false
}

func cacheHeaders(c *gin.Context, cacheExpireTime time.Time) {
	maxAge := cacheTtl.Seconds()
	age := cacheTtl.Seconds() - cacheExpireTime.Sub(time.Now()).Seconds()

	c.Header("Cache-Control", fmt.Sprintf("max-age=%.0f, public", maxAge))
	c.Header("Age", fmt.Sprintf("%.0f", age))
	c.Header("MemCache-Expires-At", cacheExpireTime.UTC().Format(time.RFC3339))
}

func readFromCache(c *gin.Context) {
	trans := apm.TransactionFromContext(c.Request.Context())

	cacheData, exists := GetFromCache(c.Request.URL.RequestURI())
	if exists {
		cacheHeaders(c, cacheData.ExpireAt)

		if trans != nil {
			trans.TransactionData.Context.SetLabel("cached", true)
		}

		c.JSON(cacheData.Status, cacheData.Data)
		c.Abort()
	} else {
		if trans != nil {
			trans.TransactionData.Context.SetLabel("cached", false)
		}
	}
}

func setTransaction(c *gin.Context) {
	trans := apm.TransactionFromContext(c.Request.Context())
	if trans != nil {
		for k, v := range c.Request.URL.Query() {
			trans.TransactionData.Context.SetLabel("query."+k, strings.ToLower(strings.Join(v, ",")))
		}

		for _, v := range c.Params {
			trans.TransactionData.Context.SetLabel("params."+v.Key, v.Value)
		}

		userAgent := c.Request.UserAgent()
		parts := strings.Split(userAgent, " ")
		for _, v := range parts {
			data := strings.Split(v, "/")
			if len(data) != 2 {
				continue
			}
			trans.TransactionData.Context.SetLabel("useragent."+data[0], data[1])
		}
	}
}

func readZipEntry(file *zip.File) ([]byte, error) {
	fileReader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer fileReader.Close()

	return io.ReadAll(fileReader)
}

func readManifest(data []byte) map[string]string {
	parsed := make(map[string]string)
	for _, v := range strings.Split(string(data), "\n") {
		split := strings.SplitN(v, ":", 2)
		if len(split) == 2 {
			parsed[strings.TrimSpace(split[0])] = strings.TrimSpace(split[1])
		}
	}

	return parsed
}
