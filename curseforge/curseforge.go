package curseforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/cfwidget/updatejson/env"
	"github.com/cfwidget/updatejson/logger"
)

const BaseUrl string = "https://api.curseforge.com/v1"
const PageSize = 50
const OverflowedPageSize = 20

var ErrUnsupportedGame = errors.New("unsupported game")
var ErrInvalidProjectId = errors.New("invalid project id")
var ErrUnauthorized = errors.New("unauthorized")
var _client *http.Client

func init() {
	_client = &http.Client{}
}

func GetProject(projectId uint, ctx context.Context) (Project, error) {
	response, err := Call(fmt.Sprintf("mods/%d", projectId), ctx)
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

func GetFilesForProject(projectId uint, ctx context.Context) ([]File, error) {
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
	response, err := Call(fmt.Sprintf("mods/%d/files?index=%d&pageSize=%d", projectId, page*PageSize, PageSize), ctx)
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

func Call(requestUri string, ctx context.Context) (*http.Response, error) {
	key := env.Get("CORE_KEY")

	p, err := url.JoinPath(BaseUrl, requestUri)
	if err != nil {
		return nil, err
	}
	path, err := url.Parse(p)
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		Method: "GET",
		URL:    path,
		Header: http.Header{},
	}
	request.Header.Add("x-api-key", key)

	if env.GetBool("DEBUG") {
		logger.FromContext(ctx).Printf("[GET] %s\n", path.String())
	}

	return _client.Do(request.WithContext(ctx))
}

func DownloadFile(requestUrl string, ctx context.Context) (*http.Response, error) {
	path, err := url.Parse(requestUrl)
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		Method: "GET",
		URL:    path,
		Header: http.Header{},
	}

	if env.GetBool("DEBUG") {
		logger.FromContext(ctx).Printf("[GET] %s\n", path.String())
	}

	return _client.Do(request.WithContext(ctx))
}

type Response struct {
}

type FileResponse struct {
	Response
	Data       []File
	Pagination Pagination
}

type ProjectResponse struct {
	Response
	Data Project
}

type File struct {
	Id           uint
	FileDate     time.Time
	DownloadUrl  string
	ReleaseType  int8
	FileStatus   int
	IsAvailable  bool
	GameVersions []string
	Modules      []Module
}

type Project struct {
	Id     uint
	GameId int
	Links  Links
}

type Links struct {
	WebsiteUrl string
	WikiUrl    string
	IssuesUrl  string
	SourceUrl  string
}

type Module struct {
	Name        string
	Fingerprint uint64
}

type Pagination struct {
	Index       int
	PageSize    int
	ResultCount int
	TotalCount  int
}
