package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cast"
	"go.elastic.co/apm/module/apmgin/v2"
	"go.elastic.co/apm/v2"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const PageSize = 50
const OverflowedPageSize = 20

var ErrUnsupportedGame = errors.New("unsupported game")
var ErrInvalidProjectId = errors.New("invalid project id")
var ErrUnauthorized = errors.New("unauthorized")

var invalidGameVersionRegex = regexp.MustCompile("[^0-9.]")

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

	_ = apm.DefaultTracer()

	r := gin.Default()
	r.Use(apmgin.Middleware(r))

	r.Use(Recover)
	r.GET("/:projectId/:modId", setTransaction, readFromCache, processRequest)
	r.GET("/:projectId/:modId/references", setTransaction, readFromCache, getReferences)
	r.GET("/:projectId/:modId/expire", setTransaction, expireCache)

	fs := http.FS(webAssets)
	r.StaticFileFS("/", "home.html", fs)
	r.GET("/service-worker.js", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	r.GET("/service-worker-dev.js", func(c *gin.Context) { c.Status(http.StatusNotFound) })

	bundledFiles, err := webAssets.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, v := range bundledFiles {
		if v.IsDir() {
			continue
		}
		r.StaticFileFS("/"+v.Name(), v.Name(), fs)
	}

	//seed certain ids just in case
	//to avoid issues at runtime where things started up and we get a request, just pre-seed the records we want before
	//we accept requests

	for _, v := range strings.Split(os.Getenv("PRESEED"), ",") {
		if v == "" {
			continue
		}

		path := strings.Split(v, ":")
		projectId := cast.ToInt(path[0])
		modId := path[1]

		for _, loader := range []string{"forge", "fabric", "neoforge", "quilt"} {
			log.Printf("Preseeding %d:%s (%s)", projectId, modId, loader)
			data, err := getUpdateJson(projectId, modId, loader, context.Background())
			if err != nil {
				log.Printf("Error refreshing project: %s", err.Error())
				continue
			}
			_ = SetInCache(fmt.Sprintf("%s.%s/%d/%s", loader, os.Getenv("HOST"), projectId, modId), http.StatusOK, *data)
			_ = SetInCache(fmt.Sprintf("%s/%d/%s?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, *data)
			_ = SetInCache(fmt.Sprintf("forge.%s/%d/%s?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, *data)

			_ = SetInCache(fmt.Sprintf("%s.%s/%d/%s/references", loader, os.Getenv("HOST"), projectId, modId), http.StatusOK, data.References)
			_ = SetInCache(fmt.Sprintf("%s/%d/%s/references?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, data.References)
			_ = SetInCache(fmt.Sprintf("forge.%s/%d/%s/references?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, data.References)
		}
	}

	log.Printf("Starting web services\n")
	err = r.Run()
	if err != nil {
		panic(err)
	}
}

func Recover(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": err,
			})
		}
	}()

	c.Next()
}

func processRequest(c *gin.Context) {
	pid := c.Param("projectId")
	modId := c.Param("modId")
	loader := getLoader(c)

	var projectId int
	var err error
	if projectId, err = strconv.Atoi(pid); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	data, err := getUpdateJson(projectId, modId, loader, c.Request.Context())
	cacheKey := buildUrl(c)

	if errors.Is(err, ErrInvalidProjectId) || errors.Is(err, ErrUnsupportedGame) {
		d := map[string]string{"error": err.Error()}
		SetInCache(cacheKey, http.StatusOK, d)
		c.JSON(http.StatusBadRequest, d)
	} else if err != nil {
		log.Printf("Error: %s", err.Error())
		d := map[string]string{"error": err.Error()}
		cacheExpireTime := SetInCache(cacheKey, http.StatusInternalServerError, d)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusInternalServerError)
	} else if data != nil {
		cacheExpireTime := SetInCache(cacheKey, http.StatusOK, *data)
		cacheHeaders(c, cacheExpireTime)
		c.JSON(http.StatusOK, data)
	} else {
		cacheExpireTime := SetInCache(cacheKey, http.StatusNotFound, nil)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusNotFound)
	}
}

func expireCache(c *gin.Context) {
	basePath := buildUrl(c)
	basePath = strings.TrimSuffix(basePath, "/expire")

	key := basePath
	if c.Request.URL.RawQuery != "" {
		key = key + "?" + c.Request.URL.RawQuery
	}
	RemoveFromCache(key)

	key = basePath + "/references"
	if c.Request.URL.RawQuery != "" {
		key = key + "?" + c.Request.URL.RawQuery
	}
	RemoveFromCache(key)

	c.Status(http.StatusAccepted)
}

func getReferences(c *gin.Context) {
	pid := c.Param("projectId")
	modId := c.Param("modId")
	loader := getLoader(c)

	cacheKey := buildUrl(c)

	var projectId int
	var err error
	if projectId, err = strconv.Atoi(pid); err != nil {
		SetInCache(cacheKey, http.StatusNotFound, nil)
		c.Status(http.StatusNotFound)
		return
	}

	data, err := getUpdateJson(projectId, modId, loader, c.Request.Context())

	if errors.Is(err, ErrInvalidProjectId) || errors.Is(err, ErrUnsupportedGame) {
		d := map[string]string{"error": err.Error()}
		SetInCache(cacheKey, http.StatusOK, d)
		c.JSON(http.StatusBadRequest, d)
	} else if err != nil {
		log.Printf("Error: %s", err.Error())
		d := map[string]string{"error": err.Error()}
		cacheExpireTime := SetInCache(cacheKey, http.StatusInternalServerError, d)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusInternalServerError)
	} else if data != nil {
		cacheExpireTime := SetInCache(cacheKey, http.StatusOK, data.References)
		cacheHeaders(c, cacheExpireTime)
		c.JSON(http.StatusOK, data.References)
	} else {
		cacheExpireTime := SetInCache(cacheKey, http.StatusNotFound, nil)
		cacheHeaders(c, cacheExpireTime)
		c.Status(http.StatusNotFound)
	}
}

func getUpdateJson(projectId int, modId string, loader string, ctx context.Context) (*UpdateJson, error) {
	project, err := getProject(projectId, ctx)
	if err != nil && !errors.Is(err, ErrUnauthorized) {
		return nil, err
	}

	if project.GameId != 432 {
		return nil, ErrUnsupportedGame
	}

	versionMap := make(map[uint]*Version)

	curseforgeFiles, err := getFiles(project.Id, ctx)
	if errors.Is(err, ErrUnauthorized) {
		//use our DB to pull what we know
		db, err := Database(ctx)
		if err != nil {
			return nil, err
		}

		var versions []*Version

		err = db.Where(&Version{CurseId: project.Id}).Find(&versions).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		for _, v := range versions {
			versionMap[v.FileId] = v
		}
	} else if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var writer sync.Mutex
	for _, v := range curseforgeFiles {
		wg.Add(1)
		go func(file File) {
			defer wg.Done()
			versionInfo, err := getModVersion(project, file, modId, ctx)
			if err != nil {
				log.Printf("Error getting mod version from file: %s", err.Error())
				return
			}
			writer.Lock()
			defer writer.Unlock()
			if versionInfo != nil {
				versionMap[file.Id] = versionInfo
			}
		}(v)
	}
	wg.Wait()

	results := make(map[string]*Version)

	for _, v := range versionMap {
		if v.ModId == modId && v.Version != "" && contains(strings.ToLower(loader), strings.Split(strings.ToLower(v.Loader), ",")) {
			gameVersions := strings.Split(v.GameVersions, ",")
			for _, version := range gameVersions {
				if invalidGameVersionRegex.MatchString(version) {
					continue
				}
				key := version + "-latest"
				existing, exists := results[key]
				if !exists {
					results[key] = v
				} else if v.ReleaseDate.After(existing.ReleaseDate) {
					results[key] = v
				}

				if v.Type == 1 {
					key = version + "-recommended"
					existing, exists = results[key]
					if !exists {
						results[key] = v
					} else if v.ReleaseDate.After(existing.ReleaseDate) {
						results[key] = v
					}
				}
			}
		}
	}

	promos := &UpdateJson{
		Promos:     map[string]string{},
		References: map[string]string{},
		HomePage:   project.Links.WebsiteUrl,
	}

	for k, v := range results {
		version, exists := versionMap[v.FileId]
		if !exists || version == nil {
			continue
		}
		if version.ModId == modId && version.Version != "" {
			promos.Promos[k] = version.Version
			promos.References[k] = version.Url
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

	if response.StatusCode == http.StatusNotFound {
		return Project{}, ErrInvalidProjectId
	}
	if response.StatusCode != http.StatusOK {
		return Project{
			Id:     uint(projectId),
			GameId: 432,
			Links:  Links{},
		}, ErrUnauthorized
	}

	var project ProjectResponse
	err = json.NewDecoder(response.Body).Decode(&project)
	return project.Data, err
}

func getFiles(projectId uint, ctx context.Context) ([]File, error) {
	files := make([]File, 0)
	page := uint(0)

	for page < OverflowedPageSize {
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

func getFilesForPage(projectId, page uint, ctx context.Context) (FileResponse, error) {
	url := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?index=%d&pageSize=%d", projectId, page*PageSize, PageSize)

	response, err := callCurseForge(url, ctx)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return FileResponse{}, ErrInvalidProjectId
	}

	if response.StatusCode != http.StatusOK {
		return FileResponse{}, ErrUnauthorized
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
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return version, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || version.Id == 0 {
		reader, size, err := downloadFile(curseFile.DownloadUrl, ctx)
		if err != nil {
			return version, err
		}

		r, err := zip.NewReader(reader, size)
		if err != nil {
			return version, err
		}

		var manifestVersion string
		manifestVersion, _ = getManifestVersion(r)

		var modInfo *ModInfo
		modInfo = parseJarFile(r, ctx)

		//update info if manifest has a version
		if modInfo != nil && manifestVersion != "" {
			for k, v := range modInfo.Mods {
				if v.Version == "${file.jarVersion}" {
					modInfo.Mods[k] = Mod{
						ModId:   v.ModId,
						Version: manifestVersion,
					}
				}
			}
		}

		if modInfo != nil {
			if modInfo.ModLoader == "forge" {
				if contains("NeoForge", curseFile.GameVersions) {
					modInfo.ModLoader = "forge,neoforge"
				}
			} else if modInfo.ModLoader == "fabric" {
				if contains("Quilt", curseFile.GameVersions) {
					modInfo.ModLoader = "fabric,quilt"
				}
			}
		}

		version.ReleaseDate = curseFile.FileDate
		version.Type = curseFile.ReleaseType

		if modInfo != nil && len(modInfo.Mods) > 0 {
			var matchingVersion *Version
			for _, z := range modInfo.Mods {
				version.Id = 0 //resets the id so we can create a new row for this mod id
				version.Version = z.Version
				version.ModId = z.ModId
				version.Url = fmt.Sprintf("%s/files/%d", project.Links.WebsiteUrl, curseFile.Id)
				version.GameVersions = strings.Join(curseFile.GameVersions, ",")
				version.Loader = modInfo.ModLoader
				err = db.Create(version).Error
				if err != nil {
					return version, err
				}
				if version.ModId == modId {
					matchingVersion = version
				}
			}

			return matchingVersion, err
		}

		//create with no real data, because it doesn't exist
		version.Url = fmt.Sprintf("%s/files/%d", project.Links.WebsiteUrl, curseFile.Id)
		version.GameVersions = strings.Join(curseFile.GameVersions, ",")

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

func parseJarFile(file *zip.Reader, ctx context.Context) *ModInfo {
	var result *ModInfo
	for _, f := range file.File {
		info := checkZipFile(f, ctx)
		if info != nil {
			if result == nil {
				result = info
			} else {
				result.Mods = append(result.Mods, info.Mods...)
				result.ModLoader = result.ModLoader + "," + info.ModLoader
			}
		}
	}

	if result != nil {
		existingLoaders := strings.Split(result.ModLoader, ",")
		result.ModLoader = strings.Join(dedup(existingLoaders), ",")
		result.Mods = dedup(result.Mods)
	}

	return result
}

func checkZipFile(file *zip.File, ctx context.Context) *ModInfo {
	var modInfo *ModInfo
	if file.Name == "META-INF/mods.toml" {
		data, err := readZipEntry(file)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo
		}

		modInfo = &ModInfo{}
		err = toml.Unmarshal(data, modInfo)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return nil
		}

		//see if the deps tell us which one is needed, ignore the mod id though...
		for _, v := range modInfo.Dependencies {
			for _, z := range v {
				if z.ModId == "forge" || z.ModId == "neoforge" {
					modInfo.ModLoader = z.ModId
					break
				}
			}
			if modInfo.ModLoader != "" {
				break
			}
		}

		//default to forge at this point
		if modInfo.ModLoader == "" {
			modInfo.ModLoader = "forge"
		}

		return modInfo
	}

	if file.Name == "fabric.mod.json" || file.Name == "quilt.mod.json" {
		data, err := readZipEntry(file)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo
		}

		var mod Mod
		err = json.Unmarshal(data, &mod)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo
		}

		modInfo = &ModInfo{Mods: []Mod{mod}}
		if file.Name == "fabric.mod.json" {
			modInfo.ModLoader = "fabric"
		}
		if file.Name == "quilt.mod.json" {
			modInfo.ModLoader = "quilt"
		}

		return modInfo
	}

	if file.Name == "mcmod.info" {
		data, err := readZipEntry(file)
		if err != nil {
			log.Printf("Error reading %s: %s", file.Name, err.Error())
			return modInfo
		}

		//there is 2 possible "variants" of the file that we can expect.
		//one is the full array, another is an object of them
		//check for the array first
		var mods []Mod
		err = json.Unmarshal(data, &mods)

		//if there is nothing in the array, assume second format
		if len(mods) == 0 {
			var alt McMod
			err = json.Unmarshal(data, &alt)
			mods = alt.ModList
		}

		if len(mods) == 0 {
			return modInfo
		}

		modInfo = &ModInfo{Mods: mods, ModLoader: "forge"}

		for k, v := range modInfo.Mods {
			modInfo.Mods[k] = Mod{
				ModId:   v.OldModId,
				Version: v.Version,
			}
		}

		return modInfo
	}

	return modInfo
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

func cacheHeaders(c *gin.Context, cacheExpireTime time.Time) {
	maxAge := cacheTtl.Seconds()
	age := cacheTtl.Seconds() - cacheExpireTime.Sub(time.Now()).Seconds()

	c.Header("Cache-Control", fmt.Sprintf("max-age=%.0f, public", maxAge))
	c.Header("Age", fmt.Sprintf("%.0f", age))
	c.Header("MemCache-Expires-At", cacheExpireTime.UTC().Format(time.RFC3339))
}

func readFromCache(c *gin.Context) {
	trans := apm.TransactionFromContext(c.Request.Context())

	cacheData, exists := GetFromCache(buildUrl(c))
	if exists {
		cacheHeaders(c, cacheData.ExpireAt)

		if trans != nil {
			trans.TransactionData.Context.SetLabel("cached", true)
		}

		if cacheData.Data != nil {
			c.JSON(cacheData.Status, cacheData.Data)
		} else {
			c.Status(cacheData.Status)
		}

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

		loader := getLoader(c)
		trans.TransactionData.Context.SetLabel("loader", loader)
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

func getLoader(c *gin.Context) string {
	loader := c.Query("ml")
	if loader != "" {
		return strings.ToLower(loader)
	}
	loader = c.Query("loader")
	if loader != "" {
		return strings.ToLower(loader)
	}

	rootHost := os.Getenv("HOST")
	if rootHost != "" {
		rootHost = "." + rootHost
		host := c.Request.Host
		if strings.HasSuffix(host, rootHost) {
			return strings.ToLower(strings.TrimSuffix(host, rootHost))
		}
	}

	return "forge"
}

func buildUrl(c *gin.Context) string {
	return c.Request.Host + c.Request.RequestURI
}

func dedup[T comparable](source []T) []T {
	result := make([]T, 0)

	for _, v := range source {
		exists := false
		for _, z := range result {
			if v == z {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, v)
		}
	}

	return result
}

func contains[T comparable](needle T, haystack []T) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}

	return false
}
