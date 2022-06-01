package main

import (
	"context"
	"github.com/cfwidget/updatejson/env"
	"go.elastic.co/apm/module/apmhttp/v2"
	"log"
	"net/http"
	"net/url"
	"time"
)

var _client *http.Client

func init() {
	_client = apmhttp.WrapClient(&http.Client{})
}

func callCurseForge(requestUrl string, ctx context.Context) (*http.Response, error) {
	key := env.Get("CORE_KEY")

	path, err := url.Parse(requestUrl)
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
		log.Printf("Calling %s\n", path.String())
	}

	return _client.Do(request.WithContext(ctx))
}

func download(requestUrl string, ctx context.Context) (*http.Response, error) {
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
		log.Printf("[OUTBOUND] [%s] %s\n", request.Method, path.String())
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
	Id           int
	FileDate     time.Time
	DownloadUrl  string
	ReleaseType  int8
	FileStatus   int
	IsAvailable  bool
	GameVersions []string
	Modules      []Module
}

type Project struct {
	Id     int
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
