package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/cfwidget/updatejson/cache"
	"github.com/cfwidget/updatejson/curseforge"
	"github.com/cfwidget/updatejson/database"
	"github.com/cfwidget/updatejson/logger"
	"github.com/cfwidget/updatejson/models"
	"github.com/cfwidget/updatejson/util"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

var invalidGameVersionRegex = regexp.MustCompile("[^0-9.]")

func main() {
	var err error

	database.Initialize()

	r := gin.Default()

	webLogger := logger.New("WEB")

	r.Use(Recover)
	r.Use(func(ctx *gin.Context) {
		ctx.Set(util.GinContextKey, context.WithValue(ctx.Request.Context(), logger.ContextKey, webLogger))
	})

	r.GET("/:projectId/:modId", readFromCache, processRequest)
	r.GET("/:projectId/:modId/references", readFromCache, getReferences)
	r.GET("/:projectId/:modId/expire", expireCache)

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

	for v := range strings.SplitSeq(os.Getenv("PRESEED"), ",") {
		if v == "" {
			continue
		}

		path := strings.Split(v, ":")
		projectId := cast.ToUint(path[0])
		modId := path[1]

		for _, loader := range []string{"forge", "fabric", "neoforge", "quilt"} {
			webLogger.Printf("Preseeding %d:%s (%s)", projectId, modId, loader)
			data, err := getUpdateJson(projectId, modId, loader, context.Background())
			if err != nil {
				webLogger.Printf("Error refreshing project: %s", err.Error())
				continue
			}
			_ = cache.Set(fmt.Sprintf("%s.%s/%d/%s", loader, os.Getenv("HOST"), projectId, modId), http.StatusOK, *data)
			_ = cache.Set(fmt.Sprintf("%s/%d/%s?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, *data)
			_ = cache.Set(fmt.Sprintf("forge.%s/%d/%s?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, *data)
			_ = cache.Set(fmt.Sprintf("%s.%s/%d/%s?ml=%s", loader, os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, *data)

			_ = cache.Set(fmt.Sprintf("%s.%s/%d/%s/references", loader, os.Getenv("HOST"), projectId, modId), http.StatusOK, data.References)
			_ = cache.Set(fmt.Sprintf("%s/%d/%s/references?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, data.References)
			_ = cache.Set(fmt.Sprintf("forge.%s/%d/%s/references?ml=%s", os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, data.References)
			_ = cache.Set(fmt.Sprintf("%s.%s/%d/%s/references?ml=%s", loader, os.Getenv("HOST"), projectId, modId, loader), http.StatusOK, data.References)
		}
	}

	webLogger.Printf("Starting web services\n")
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

	var projectId uint
	var err error
	if projectId, err = cast.ToUintE(pid); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	var data *models.UpdateJson
	data, err = getUpdateJson(projectId, modId, loader, c.Request.Context())
	cacheKey := cache.GetKey(c)

	if errors.Is(err, curseforge.ErrInvalidProjectId) || errors.Is(err, curseforge.ErrUnsupportedGame) {
		d := map[string]string{"error": err.Error()}
		cache.Set(cacheKey, http.StatusBadRequest, d)
		c.JSON(http.StatusBadRequest, d)
	} else if err != nil {
		logger.Printf(c.Request.Context(), "Error: %s", err.Error())
		d := map[string]string{"error": err.Error()}
		cacheExpireTime := cache.Set(cacheKey, http.StatusInternalServerError, d)
		cache.AddHeaders(c, cacheExpireTime)
		c.Status(http.StatusInternalServerError)
	} else if data != nil {
		cacheExpireTime := cache.Set(cacheKey, http.StatusOK, *data)
		cache.AddHeaders(c, cacheExpireTime)
		c.JSON(http.StatusOK, data)
	} else {
		cacheExpireTime := cache.Set(cacheKey, http.StatusNotFound, nil)
		cache.AddHeaders(c, cacheExpireTime)
		c.Status(http.StatusNotFound)
	}
}

func expireCache(c *gin.Context) {
	basePath := cache.GetKey(c)
	basePath = strings.TrimSuffix(basePath, "/expire")

	key := basePath
	if c.Request.URL.RawQuery != "" {
		key = key + "?" + c.Request.URL.RawQuery
	}
	cache.Remove(key)

	key = basePath + "/references"
	if c.Request.URL.RawQuery != "" {
		key = key + "?" + c.Request.URL.RawQuery
	}
	cache.Remove(key)

	c.Status(http.StatusAccepted)
}

func getReferences(c *gin.Context) {
	pid := c.Param("projectId")
	modId := c.Param("modId")
	loader := getLoader(c)

	cacheKey := cache.GetKey(c)

	var projectId uint
	var err error
	if projectId, err = cast.ToUintE(pid); err != nil {
		cache.Set(cacheKey, http.StatusNotFound, nil)
		c.Status(http.StatusNotFound)
		return
	}

	var data *models.UpdateJson
	data, err = getUpdateJson(projectId, modId, loader, c.Request.Context())

	if errors.Is(err, curseforge.ErrInvalidProjectId) || errors.Is(err, curseforge.ErrUnsupportedGame) {
		d := map[string]string{"error": err.Error()}
		cache.Set(cacheKey, http.StatusOK, d)
		c.JSON(http.StatusBadRequest, d)
	} else if err != nil {
		logger.Printf(c.Request.Context(), "Error: %s", err.Error())
		d := map[string]string{"error": err.Error()}
		cacheExpireTime := cache.Set(cacheKey, http.StatusInternalServerError, d)
		cache.AddHeaders(c, cacheExpireTime)
		c.Status(http.StatusInternalServerError)
	} else if data != nil {
		cacheExpireTime := cache.Set(cacheKey, http.StatusOK, data.References)
		cache.AddHeaders(c, cacheExpireTime)
		c.JSON(http.StatusOK, data.References)
	} else {
		cacheExpireTime := cache.Set(cacheKey, http.StatusNotFound, nil)
		cache.AddHeaders(c, cacheExpireTime)
		c.Status(http.StatusNotFound)
	}
}

func getUpdateJson(projectId uint, modId string, loader string, ctx context.Context) (*models.UpdateJson, error) {
	project, err := curseforge.GetProject(projectId, ctx)
	if err != nil && !errors.Is(err, curseforge.ErrUnauthorized) {
		return nil, err
	}

	if project.GameId != 432 {
		return nil, curseforge.ErrUnsupportedGame
	}

	versionMap := make(map[uint]*models.Version)

	var curseforgeFiles []curseforge.File
	curseforgeFiles, err = curseforge.GetFilesForProject(project.Id, ctx)
	if errors.Is(err, curseforge.ErrUnauthorized) {
		//use our DB to pull what we know
		var db *gorm.DB
		db, err = database.Get(ctx)
		if err != nil {
			return nil, err
		}

		var versions []*models.Version

		err = db.Where(&models.Version{CurseId: project.Id}).Find(&versions).Error
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
		downloaderWorkerQueue <- &QueueItem{
			File:       v,
			Wg:         &wg,
			Mutex:      &writer,
			VersionMap: versionMap,
			Ctx:        ctx,
			Project:    project,
			ModId:      modId,
		}
	}
	wg.Wait()

	results := make(map[string]*models.Version)

	for _, v := range versionMap {
		if v.ModId == modId && v.Version != "" && slices.Contains(strings.Split(strings.ToLower(v.Loader), ","), strings.ToLower(loader)) {
			gameVersions := strings.SplitSeq(v.GameVersions, ",")
			for version := range gameVersions {
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

	promos := &models.UpdateJson{
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

func getModVersion(project curseforge.Project, curseFile curseforge.File, modId string, ctx context.Context) (*models.Version, error) {
	db, err := database.Get(ctx)
	if err != nil {
		return nil, err
	}

	version := &models.Version{
		CurseId: project.Id,
		FileId:  curseFile.Id,
	}

	err = db.Where(version).Find(&version).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return version, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || version.Id == 0 {
		reader, size, err := downloadFile(curseFile.DownloadUrl, ctx)
		defer util.Close(reader)
		if err != nil {
			return version, err
		}

		r, err := zip.NewReader(reader, size)
		if err != nil {
			return version, err
		}

		var manifestVersion string
		manifestVersion, _ = getManifestVersion(r)

		var modInfo *models.ModInfo
		modInfo = parseJarFile(r, ctx)

		//update info if manifest has a version
		if modInfo != nil && manifestVersion != "" {
			for k, v := range modInfo.Mods {
				if v.Version == "${file.jarVersion}" {
					modInfo.Mods[k] = models.Mod{
						ModId:   v.ModId,
						Version: manifestVersion,
					}
				}
			}
		}

		if modInfo != nil {
			switch modInfo.ModLoader {
			case "forge":
				if slices.Contains(curseFile.GameVersions, "NeoForge") {
					modInfo.ModLoader = "forge,neoforge"
				}
			case "fabric":
				if slices.Contains(curseFile.GameVersions, "Quilt") {
					modInfo.ModLoader = "fabric,quilt"
				}
			}
		}

		version.ReleaseDate = curseFile.FileDate
		version.Type = curseFile.ReleaseType

		if modInfo != nil && len(modInfo.Mods) > 0 {
			var matchingVersion *models.Version
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
	if !util.AreEqual(currentVersions, curseFile.GameVersions) {
		version.GameVersions = strings.Join(curseFile.GameVersions, ",")
		err = db.Save(version).Error
	}

	if version.Type != curseFile.ReleaseType {
		version.Type = curseFile.ReleaseType
		err = db.Save(version).Error
	}

	return version, err
}

func parseJarFile(file *zip.Reader, ctx context.Context) *models.ModInfo {
	var result *models.ModInfo
	for _, f := range file.File {
		info, err := checkZipFile(f, ctx)
		if err != nil {
			logger.Printf(ctx, "Failed to parse mod file %s: %s", f.Name, err)
		} else if info != nil {
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
		result.ModLoader = strings.Join(util.Dedup(existingLoaders), ",")
		result.Mods = util.Dedup(result.Mods)
	}

	return result
}

func checkZipFile(file *zip.File, ctx context.Context) (*models.ModInfo, error) {
	var modInfo *models.ModInfo
	if file.Name == "META-INF/mods.toml" {
		data, err := readZipEntry(file)
		if err != nil {
			return modInfo, err
		}

		modInfo = &models.ModInfo{}
		err = toml.Unmarshal(data, modInfo)
		if err != nil {
			return nil, err
		}
		//reset what the info actually has for the loader, because we don't care about javafml
		modInfo.ModLoader = ""

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

		return modInfo, nil
	}

	if file.Name == "META-INF/neoforge.mods.toml" {
		data, err := readZipEntry(file)
		if err != nil {
			return nil, err
		}

		modInfo = &models.ModInfo{}
		err = toml.Unmarshal(data, modInfo)
		if err != nil {
			return nil, err
		}
		//reset what the info actually has for the loader, because we don't care about javafml
		modInfo.ModLoader = "neoforge"
		return modInfo, nil
	}

	if file.Name == "fabric.mod.json" || file.Name == "quilt.mod.json" {
		data, err := readZipEntry(file)
		if err != nil {
			return nil, err
		}

		var mod models.Mod
		err = json.Unmarshal(data, &mod)
		if err != nil {
			return nil, err
		}

		modInfo = &models.ModInfo{Mods: []models.Mod{mod}}
		if file.Name == "fabric.mod.json" {
			modInfo.ModLoader = "fabric"
		}
		if file.Name == "quilt.mod.json" {
			modInfo.ModLoader = "quilt"
		}

		return modInfo, nil
	}

	if file.Name == "mcmod.info" {
		data, err := readZipEntry(file)
		if err != nil {
			return nil, err
		}

		//there is 2 possible "variants" of the file that we can expect.
		//one is the full array, another is an object of them
		//check for the array first
		var mods []models.Mod
		err = json.Unmarshal(data, &mods)

		//if there is nothing in the array, assume second format
		if len(mods) == 0 {
			var alt models.McMod
			err = json.Unmarshal(data, &alt)
			mods = alt.ModList
		}

		if len(mods) == 0 {
			return modInfo, nil
		}

		modInfo = &models.ModInfo{Mods: mods, ModLoader: "forge"}

		for k, v := range modInfo.Mods {
			modInfo.Mods[k] = models.Mod{
				ModId:   v.OldModId,
				Version: v.Version,
			}
		}

		return modInfo, nil
	}

	return modInfo, nil
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

func downloadFile(url string, ctx context.Context) (*util.TempFile, int64, error) {
	response, err := curseforge.DownloadFile(url, ctx)
	if err != nil {
		return nil, 0, err
	}

	defer response.Body.Close()

	f, err := util.NewTempFile()
	size, err := io.Copy(f, response.Body)
	if err != nil {
		return nil, 0, err
	}

	return f, size, nil
}

func readFromCache(c *gin.Context) {
	cacheData, exists := cache.GetByRequest(c)
	if exists {
		cache.AddHeaders(c, cacheData.ExpireAt)

		if cacheData.Data != nil {
			c.JSON(cacheData.Status, cacheData.Data)
		} else {
			c.Status(cacheData.Status)
		}

		c.Abort()
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
