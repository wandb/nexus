package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/wandb/wandb/nexus/pkg/observability"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/wandb/wandb/nexus/pkg/service"
)

const (
	HistoryFileName = "wandb-history.jsonl"
	maxItemsPerPush = 5_000
	delayProcess    = 20 * time.Millisecond
	heartbeatTime   = 2 * time.Second
)

var (
	exitcodeZero int  = 0
	completeTrue bool = true
)

type chunkFile int8

const (
	historyChunk chunkFile = iota
	outputChunk
)

type chunkData struct {
	fileData *chunkLine
	Exitcode *int
	Complete *bool
}

type chunkLine struct {
	chunkType chunkFile
	line      string
}

// FsChunkData is the data for a chunk of a file
type FsChunkData struct {
	Offset  int      `json:"offset"`
	Content []string `json:"content"`
}

type FsData struct {
	Files    map[string]FsChunkData `json:"files,omitempty"`
	Complete *bool                  `json:"complete,omitempty"`
	Exitcode *int                   `json:"exitcode,omitempty"`
}

// FileStream is a stream of data to the server
type FileStream struct {

	// recordChan is the channel for incoming messages
	recordChan chan *service.Record

	// chunkChan is the channel for chunk data
	chunkChan chan chunkData

	// replyChan is the channel for replies
	replyChan chan map[string]interface{}

	// wg is the wait group
	wg *sync.WaitGroup

	path string

	// FIXME this should be per db
	offset int

	// settings is the settings for the filestream
	settings *service.Settings

	//// logger is the logger for the filestream
	//logger *observability.NexusLogger

	// httpClient is the http client
	httpClient *retryablehttp.Client
}

// NewFileStream creates a new filestream
func NewFileStream(path string, settings *service.Settings, logger *observability.NexusLogger) *FileStream {
	retryClient := newRetryClient(settings.GetApiKey().GetValue(), logger)
	fs := FileStream{
		path:     path,
		settings: settings,
		//logger:     logger,
		httpClient: retryClient,
		wg:         &sync.WaitGroup{},
	}
	fs.Start()
	return &fs
}

func (fs *FileStream) Start() {
	//fs.logger.Debug("filestream: start", "path", fs.path)

	fs.recordChan = make(chan *service.Record, BufferSize)

	chunkChan := fs.doRecordProcess(fs.recordChan)
	replayChan := fs.doChunkProcess(chunkChan)

	fs.wg.Add(1)
	go func() {
		fs.doReplyProcess(replayChan)
		fs.wg.Done()
	}()
}

func (fs *FileStream) pushRecord(rec *service.Record) {
	fs.recordChan <- rec
}

func (fs *FileStream) pushChunk(chunk chunkData) {
	fs.chunkChan <- chunk
}

func (fs *FileStream) pushReply(reply map[string]interface{}) {
	fs.replyChan <- reply
}

func (fs *FileStream) doRecordProcess(inChan <-chan *service.Record) <-chan chunkData {
	//fs.logger.Debug("filestream: open", "path", fs.path)

	fs.chunkChan = make(chan chunkData, BufferSize)

	go func() {
		for record := range inChan {
			//fs.logger.Debug("filestream: record", "record", record)
			switch x := record.RecordType.(type) {
			case *service.Record_History:
				fs.streamHistory(x.History)
			case *service.Record_Exit:
				fs.streamFinish()
			case nil:
				err := fmt.Errorf("filestream: field not set")
				panic(err)
				//fs.logger.CaptureFatalAndPanic("filestream error:", err)
			default:
				err := fmt.Errorf("filestream: Unknown type %T", x)
				panic(err)
				//fs.logger.CaptureFatalAndPanic("filestream error:", err)
			}
		}
		close(fs.chunkChan)
		//fs.logger.Debug("filestream: close", "path", fs.path)
	}()
	return fs.chunkChan
}

func (fs *FileStream) doChunkProcess(inChan <-chan chunkData) <-chan map[string]interface{} {

	fs.replyChan = make(chan map[string]interface{}, BufferSize)
	go func() {
		overflow := false
		for active := true; active; {
			var chunkList []chunkData
			select {
			case chunk, ok := <-inChan:
				if !ok {
					active = false
					break
				}
				chunkList = append(chunkList, chunk)

				delayTime := delayProcess
				if overflow {
					delayTime = 0
				}
				delayChan := time.After(delayTime)
				overflow = false

				for ready := true; ready; {
					select {
					case chunk, ok := <-inChan:
						if !ok {
							ready = false
							active = false
							break
						}
						chunkList = append(chunkList, chunk)
						if len(chunkList) >= maxItemsPerPush {
							ready = false
							overflow = true
						}
					case <-delayChan:
						ready = false
					}
				}
				fs.sendChunkList(chunkList)
			case <-time.After(heartbeatTime):
				//fs.logger.Debug("filestream: heartbeat... (timeout)", "offset", fs.offset)
				fs.sendChunkList(chunkList)
			}
		}
		close(fs.replyChan)
	}()
	return fs.replyChan
}

func (fs *FileStream) doReplyProcess(inChan <-chan map[string]interface{}) {
	for range inChan {
	}
}

func (fs *FileStream) jsonifyHistory(record *service.HistoryRecord) string {
	jsonMap := make(map[string]interface{})

	for _, item := range record.Item {
		var value interface{}
		if err := json.Unmarshal([]byte(item.ValueJson), &value); err != nil {
			e := fmt.Errorf("json unmarshal error: %v, items: %v", err, item)
			//fs.logger.CaptureFatalAndPanic("json unmarshal error", e)
			panic(e)
		}
		jsonMap[item.Key] = value
	}

	jsonBytes, err := json.Marshal(jsonMap)
	if err != nil {
		//fs.logger.CaptureFatalAndPanic("json unmarshal error", err)
		panic(err)
	}
	return string(jsonBytes)
}

func (fs *FileStream) streamHistory(msg *service.HistoryRecord) {
	chunk := chunkData{fileData: &chunkLine{
		chunkType: historyChunk,
		line:      fs.jsonifyHistory(msg)},
	}
	fs.pushChunk(chunk)
}

func (fs *FileStream) streamFinish() {
	chunk := chunkData{Complete: &completeTrue, Exitcode: &exitcodeZero}
	fs.pushChunk(chunk)
}

func (fs *FileStream) StreamRecord(rec *service.Record) {
	//fs.logger.Debug("filestream: stream record", "record", rec)
	fs.pushRecord(rec)
}

func (fs *FileStream) sendChunkList(chunks []chunkData) {
	var lines []string
	var complete *bool
	var exitcode *int

	for i := range chunks {
		if chunks[i].fileData != nil {
			lines = append(lines, chunks[i].fileData.line)
		}
		if chunks[i].Complete != nil {
			complete = chunks[i].Complete
		}
		if chunks[i].Exitcode != nil {
			exitcode = chunks[i].Exitcode
		}
	}

	fsChunk := FsChunkData{
		Offset:  fs.offset,
		Content: lines}
	fs.offset += len(lines)
	chunkFileName := HistoryFileName
	var files map[string]FsChunkData
	if len(lines) > 0 {
		files = map[string]FsChunkData{
			chunkFileName: fsChunk,
		}
	}
	data := FsData{Files: files, Complete: complete, Exitcode: exitcode}
	fs.send(data)
}

func (fs *FileStream) send(data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		//fs.logger.CaptureFatalAndPanic("json marshal error", err)
		panic(err)
	}

	buffer := bytes.NewBuffer(jsonData)
	req, err := retryablehttp.NewRequest(http.MethodPost, fs.path, buffer)
	if err != nil {
		//fs.logger.CaptureFatalAndPanic("filestream: error creating HTTP request", err)
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := fs.httpClient.Do(req)
	if err != nil {
		//fs.logger.CaptureFatalAndPanic("filestream: error making HTTP request", err)
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		if err = Body.Close(); err != nil {
			//fs.logger.CaptureError("filestream: error closing response body", err)
		}
	}(resp.Body)

	var res map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		//fs.logger.CaptureError("json decode error", err)
	}
	fs.pushReply(res)

	//fs.logger.Debug("filestream: post response", "response", res)
}

func (fs *FileStream) Close() {
	//fs.logger.Debug("filestream: closing...")
	close(fs.recordChan)
	fs.wg.Wait()
}
