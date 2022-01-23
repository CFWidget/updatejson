package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var _client = &http.Client{}

func callCurseForge(requestUrl string) (*http.Response, error) {
	key := os.Getenv("CORE_KEY")

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

	if os.Getenv("DEBUG") == "true" {
		log.Printf("Calling %s\n", path.String())
	}

	return _client.Do(request)
}

type Response struct {
}

type FileResponse struct {
	Response
	Data []File
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
	ReleaseType  int
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
