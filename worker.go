package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"

	"github.com/cfwidget/updatejson/curseforge"
	"github.com/cfwidget/updatejson/env"
	"github.com/cfwidget/updatejson/logger"
	"github.com/cfwidget/updatejson/models"
)

var downloaderWorkerQueue chan *QueueItem
var workers []*Worker

func init() {
	numWorkers := env.GetInt("DOWNLOADERS")
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU() / 2
	}
	downloaderWorkerQueue = make(chan *QueueItem, numWorkers*2)
	for i := range numWorkers {
		w := &Worker{Id: i, Logger: logger.New(fmt.Sprintf("Worker-%d", i)), Stop: make(chan bool)}
		workers = append(workers, w)
		go w.Start()
	}
}

func (w *Worker) Start() {
	done := false
	for !done {
		select {
		case i := <-downloaderWorkerQueue:
			w.Logger.Printf("Downloading %s\n", i.File.DownloadUrl)
			w.ProcessItem(i)
		case <-w.Stop:
			done = true
		}
	}
}

type Worker struct {
	Id     int
	Logger *log.Logger
	Stop   chan bool
}

func (w *Worker) ProcessItem(item *QueueItem) {
	defer item.Wg.Done()
	ctx := context.WithValue(item.Ctx, logger.ContextKey, w.Logger)
	versionInfo, err := getModVersion(item.Project, item.File, item.ModId, ctx)
	if err != nil {
		w.Logger.Printf("Error getting mod version from file: %s", err.Error())
		return
	}
	item.Mutex.Lock()
	defer item.Mutex.Unlock()
	if versionInfo != nil {
		item.VersionMap[item.File.Id] = versionInfo
	}
}

type QueueItem struct {
	File       curseforge.File
	Wg         *sync.WaitGroup
	Mutex      *sync.Mutex
	VersionMap map[uint]*models.Version
	Ctx        context.Context
	Project    curseforge.Project
	ModId      string
}
