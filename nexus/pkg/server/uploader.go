package server

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/exp/slog"

	"github.com/hashicorp/go-retryablehttp"
)

type UploadTask struct {
	path string
	url  string
}

type Uploader struct {
	ctx         context.Context
	inChan      chan *UploadTask
	retryClient *retryablehttp.Client
	logger      *slog.Logger
	wg          *sync.WaitGroup
}

func NewUploader(ctx context.Context, logger *slog.Logger) *Uploader {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 60 * time.Second

	uploader := &Uploader{
		ctx:         ctx,
		inChan:      make(chan *UploadTask),
		retryClient: retryClient,
		logger:      logger,
		wg:          &sync.WaitGroup{},
	}
	uploader.wg.Add(1)
	go uploader.start()
	return uploader
}

func (u *Uploader) start() {
	defer u.wg.Done()

	u.logger.Debug("uploader: start")
	for task := range u.inChan {
		u.logger.Debug("uploader: got task", task)
		u.upload(task)
	}
}

func (u *Uploader) addTask(task *UploadTask) {
	u.logger.Debug("uploader: adding task, %v", task)
	u.inChan <- task
}

func (u *Uploader) close() {
	u.logger.Debug("uploader: Close")
	close(u.inChan)
	u.wg.Wait()
}

func (u *Uploader) upload(task *UploadTask) error {
	// read in the file at task.path:
	file, err := os.ReadFile(task.path)
	if err != nil {
		return err
	}

	req, err := retryablehttp.NewRequest(
		http.MethodPut,
		task.url,
		file,
	)
	if err != nil {
		return err
	}

	_, err = u.retryClient.Do(req)
	if err != nil {
		return err
	}
	return nil
}
