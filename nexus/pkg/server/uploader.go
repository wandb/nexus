package server

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type UploadTask struct {
	path string
	url  *url.URL
}

type Uploader struct {
	ctx         context.Context
	inChan      chan *UploadTask
	retryClient *retryablehttp.Client
}

func NewUploader(ctx context.Context) *Uploader {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 10
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 60 * time.Second

	uploader := &Uploader{
		ctx:         ctx,
		inChan:      make(chan *UploadTask),
		retryClient: retryClient,
	}
	return uploader
}

func (u *Uploader) Do() {
	for task := range u.inChan {
		go u.upload(task)
	}
}

func (u *Uploader) upload(task *UploadTask) {
	// read in the file at task.path:
	file, err := os.ReadFile(task.path)
	if err != nil {
		panic(err)
	}

	req, err := retryablehttp.NewRequest(
		http.MethodPut,
		task.url.String(),
		file,
	)
	if err != nil {
		panic(err)
	}

	_, err = u.retryClient.Do(req)
	if err != nil {
		panic(err)
	}
}
