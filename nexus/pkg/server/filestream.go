package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/wandb/wandb/nexus/pkg/service"

	"golang.org/x/exp/slog"
)

const HistoryFileName = "wandb-history.jsonl"
const maxItemsPerPush = 5_000
const delayProcess = 20 * time.Millisecond
const heartbeatTime = 2 * time.Second

type chunkFile int8
const (
	historyChunk chunkFile = iota
	outputChunk
)

type chunkLine struct {
	chunkType chunkFile
	line string
}

type FileStream struct {
	recordChan chan *service.Record
	recordWait *sync.WaitGroup
	chunkChan  chan chunkLine
	chunkWait  *sync.WaitGroup
	replyChan  chan map[string]interface{}
	replyWait  *sync.WaitGroup

	path   string

	// FIXME this should be per db
	offset int

	settings   *service.Settings
	logger     *slog.Logger
	httpClient *http.Client
}

func NewFileStream(path string, settings *service.Settings, logger *slog.Logger) *FileStream {
	httpClient := newHttpClient(settings.GetApiKey().GetValue())
	fs := FileStream{
		path:     path,
		settings: settings,
		logger:   logger,
		httpClient: &httpClient,
		recordChan: make(chan *service.Record),
		recordWait: &sync.WaitGroup{},
		chunkChan:  make(chan chunkLine),
		chunkWait:  &sync.WaitGroup{},
		replyChan:  make(chan map[string]interface{}),
		replyWait:  &sync.WaitGroup{},
	}
	return &fs
}

func (fs *FileStream) Start() {
	fs.recordWait.Add(1)
	go fs.recordProcess()
	fs.chunkWait.Add(1)
	go fs.chunkProcess()
	fs.replyWait.Add(1)
	go fs.replyProcess()
}

func (fs *FileStream) pushRecord(rec *service.Record) {
	fs.recordChan <- rec
}

func (fs *FileStream) pushChunk(chunk chunkLine) {
	fs.chunkChan <- chunk
}

func (fs *FileStream) pushReply(reply map[string]interface{}) {
	fs.replyChan <- reply
}

func (fs *FileStream) recordProcess() {
	defer fs.recordWait.Done()

	fs.logger.Debug("FileStream: OPEN")

	if fs.settings.GetXOffline().GetValue() {
		return
	}

	for msg := range fs.recordChan {
		fs.logger.Debug("FileStream *******")
		LogRecord(fs.logger, "FileStream: got record", msg)
		fs.streamRecord(msg)
	}
	fs.logger.Debug("FileStream: finished")
}

func (fs *FileStream) chunkProcess() {
	defer fs.chunkWait.Done()
	overflow := false

	for active := true; active; {
		var chunkList []chunkLine
		select {
		case chunk, ok := <-fs.chunkChan:
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

			for ready := true; ready;{
				select {
				case chunk, ok := <-fs.chunkChan:
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
			fmt.Println("timeout")
			fs.sendChunkList(chunkList)
		}
	}
}

func (fs *FileStream) replyProcess() {
	defer fs.replyWait.Done()
	for _ = range fs.replyChan {
	}
}

func (fs *FileStream) streamRecord(msg *service.Record) {
	switch x := msg.RecordType.(type) {
	case *service.Record_History:
		fs.streamHistory(x.History)
	case *service.Record_Exit:
		fs.streamFinish()
	case nil:
		// The field is not set.
		LogFatal(fs.logger, "FileStream: RecordType is nil")
	default:
		LogFatal(fs.logger, fmt.Sprintf("FileStream: Unknown type %T", x))
	}
}

func (fs *FileStream) streamHistory(msg *service.HistoryRecord) {
	fs.pushChunk(chunkLine{chunkType: historyChunk, line: jsonifyHistory(msg, fs.logger)})
}

func (fs *FileStream) streamFinish() {
	type FsFinishedData struct {
		Complete bool `json:"complete"`
		Exitcode int  `json:"exitcode"`
	}

	data := FsFinishedData{Complete: true, Exitcode: 0}
	fs.send(data)
}

func (fs *FileStream) StreamRecord(rec *service.Record) {
	if fs.settings.GetXOffline().GetValue() {
		return
	}
	LogRecord(fs.logger, "+++++FileStream: stream", rec)
	fs.pushRecord(rec)
}

type FsChunkData struct {
	Offset  int      `json:"offset"`
	Content []string `json:"content"`
}

type FsFilesData struct {
	Files map[string]FsChunkData `json:"files"`
}

func (fs *FileStream) sendChunkList(chunks []chunkLine) {
	var lines []string
	for i := range chunks {
		lines = append(lines, chunks[i].line)
	}

	fsChunk := FsChunkData{
		Offset:  fs.offset,
		Content: lines}
	fs.offset += len(lines)
	chunkFileName := HistoryFileName
	files := map[string]FsChunkData{
		chunkFileName: fsChunk,
	}
	data := FsFilesData{Files: files}
	fs.send(data)
}

func (fs *FileStream) send(data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		LogFatalError(fs.logger, "json marshal error", err)
	}

	buffer := bytes.NewBuffer(jsonData)
	req, err := http.NewRequest(http.MethodPost, fs.path, buffer)
	if err != nil {
		LogFatalError(fs.logger, "FileStream: could not create request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := fs.httpClient.Do(req)
	if err != nil {
		LogFatalError(fs.logger, "FileStream: error making HTTP request", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			LogError(fs.logger, "FileStream: error closing response body", err)
		}
	}(resp.Body)

	var res map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		LogError(fs.logger, "json decode error", err)
	}
	fs.pushReply(res)

	/*
	{"time":"2023-07-07T00:54:18.788638-04:00","level":"DEBUG","msg":"FileStream: post response: map[exitcode:0 limits:map[code_saving_enabled:true gpu_enabled:1.560566754e+09 hub_settings:map[disk:20Gi docker_enabled:false expiration:2.592e+06 image:<nil> redis_enabled:false repo:wandb/simpsons] name:default private_projects:false rate_limit:400/s restricted:false sweeps_enabled:false system_metrics:2/m teams_enabled:false]]"}
	*/
	fs.logger.Debug(fmt.Sprintf("FileStream: post response: %v", res))
}

func jsonifyHistory(msg *service.HistoryRecord, logger *slog.Logger) string {
	data := make(map[string]interface{})

	for _, item := range msg.Item {
		var val interface{}
		if err := json.Unmarshal([]byte(item.ValueJson), &val); err != nil {
			LogFatal(logger, fmt.Sprintf("json unmarshal error: %v, items: %v", err, item))
		}
		data[item.Key] = val
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		LogError(logger, "json marshal error", err)
	}
	return string(jsonData)
}

func (fs *FileStream) Close() {
	fs.logger.Debug("FileStream: CLOSE")
	close(fs.recordChan)
	fs.recordWait.Wait()
	close(fs.chunkChan)
	fs.chunkWait.Wait()
	close(fs.replyChan)
	fs.replyWait.Wait()
}
