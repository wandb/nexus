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
	EventsFileName  = "wandb-events.jsonl"
	HistoryFileName = "wandb-history.jsonl"
	OutputFileName  = "output.log"
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
	eventsChunk
)

type chunkData struct {
	fileName string
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
	recordWait *sync.WaitGroup
	chunkWait  *sync.WaitGroup
	replyWait  *sync.WaitGroup

	path string

	// FIXME this should be per db
	offset int

	// settings is the settings for the filestream
	settings *service.Settings

	// logger is the logger for the filestream
	logger *observability.NexusLogger

	// httpClient is the http client
	httpClient *retryablehttp.Client
}

// NewFileStream creates a new filestream
func NewFileStream(path string, settings *service.Settings, logger *observability.NexusLogger) *FileStream {
	retryClient := newRetryClient(settings.GetApiKey().GetValue(), logger)
	fs := FileStream{
		path:       path,
		settings:   settings,
		logger:     logger,
		httpClient: retryClient,
		recordWait: &sync.WaitGroup{},
		chunkWait:  &sync.WaitGroup{},
		replyWait:  &sync.WaitGroup{},
		recordChan: make(chan *service.Record, BufferSize),
		chunkChan:  make(chan chunkData, BufferSize),
		replyChan:  make(chan map[string]interface{}, BufferSize),
	}
	fs.Start()
	return &fs
}

func (fs *FileStream) Start() {
	fs.logger.Debug("filestream: start", "path", fs.path)

	fs.recordWait.Add(1)
	go func() {
		fs.doRecordProcess(fs.recordChan)
		fs.recordWait.Done()
	}()

	fs.chunkWait.Add(1)
	go func() {
		fs.doChunkProcess(fs.chunkChan)
		fs.chunkWait.Done()
	}()

	fs.replyWait.Add(1)
	go func() {
		fs.doReplyProcess(fs.replyChan)
		fs.replyWait.Done()
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

func (fs *FileStream) doRecordProcess(inChan <-chan *service.Record) {
	fs.logger.Debug("filestream: open", "path", fs.path)

	for record := range inChan {
		fs.logger.Debug("filestream: record", "record", record)
		switch x := record.RecordType.(type) {
		case *service.Record_History:
			fs.streamHistory(x.History)
		case *service.Record_Stats:
			fs.streamSystemMetrics(x.Stats)
		case *service.Record_OutputRaw:
			fs.streamOutputRaw(x.OutputRaw)
		case *service.Record_Exit:
			fs.streamFinish()
		case nil:
			err := fmt.Errorf("filestream: field not set")
			fs.logger.CaptureFatalAndPanic("filestream error:", err)
		default:
			err := fmt.Errorf("filestream: Unknown type %T", x)
			fs.logger.CaptureFatalAndPanic("filestream error:", err)
		}
	}
}

func (fs *FileStream) doChunkProcess(inChan <-chan chunkData) {

	overflow := false

	for active := true; active; {
		var chunkMaps = make(map[string][]chunkData)
		select {
		case chunk, ok := <-inChan:
			if !ok {
				active = false
				break
			}

			if chunk.fileName == EventsFileName {
				fs.sendChunkList([]chunkData{chunk})
				continue
			}

			chunkMaps[chunk.fileName] = append(chunkMaps[chunk.fileName], chunk)

			delayTime := delayProcess
			if overflow {
				delayTime = 0
			}
			delayChan := time.After(delayTime)
			overflow = false

			for ready := true; ready; {
				select {
				case chunk, ok = <-inChan:
					if !ok {
						ready = false
						active = false
						break
					}
					chunkMaps[chunk.fileName] = append(chunkMaps[chunk.fileName], chunk)
					if len(chunkMaps[chunk.fileName]) >= maxItemsPerPush {
						ready = false
						overflow = true
					}
				case <-delayChan:
					ready = false
				}
			}
			for _, chunkList := range chunkMaps {
				fs.sendChunkList(chunkList)
			}
		case <-time.After(heartbeatTime):
			for _, chunkList := range chunkMaps {
				if len(chunkList) > 0 {
					fs.sendChunkList(chunkList)
				}
			}
		}
	}
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
			fs.logger.CaptureFatalAndPanic("json unmarshal error", e)
		}
		jsonMap[item.Key] = value
	}

	jsonBytes, err := json.Marshal(jsonMap)
	if err != nil {
		fs.logger.CaptureFatalAndPanic("json unmarshal error", err)
	}
	return string(jsonBytes)
}

func (fs *FileStream) streamHistory(msg *service.HistoryRecord) {
	chunk := chunkData{
		fileName: HistoryFileName,
		fileData: &chunkLine{
			chunkType: historyChunk,
			line:      fs.jsonifyHistory(msg),
		},
	}
	fs.pushChunk(chunk)
}

func (fs *FileStream) streamOutputRaw(msg *service.OutputRawRecord) {
	chunk := chunkData{
		fileName: OutputFileName,
		fileData: &chunkLine{
			chunkType: outputChunk,
			line:      msg.Line,
		},
	}
	fs.pushChunk(chunk)
}

func (fs *FileStream) streamSystemMetrics(msg *service.StatsRecord) {
	// todo: there is a lot of unnecessary overhead here,
	//  we should prepare all the data in the system monitor
	//  and then send it in one record
	row := make(map[string]interface{})
	row["_wandb"] = true
	timestamp := float64(msg.GetTimestamp().Seconds) + float64(msg.GetTimestamp().Nanos)/1e9
	row["_timestamp"] = timestamp
	row["_runtime"] = timestamp - fs.settings.XStartTime.GetValue()

	for _, item := range msg.Item {
		var val interface{}
		if err := json.Unmarshal([]byte(item.ValueJson), &val); err != nil {
			e := fmt.Errorf("json unmarshal error: %v, items: %v", err, item)
			errMsg := fmt.Sprintf("sender: sendSystemMetrics: failed to marshal value: %s for key: %s", item.ValueJson, item.Key)
			fs.logger.CaptureError(errMsg, e)
			continue
		}

		row["system."+item.Key] = val
	}

	// marshal the row
	line, err := json.Marshal(row)
	if err != nil {
		fs.logger.CaptureError("sender: sendSystemMetrics: failed to marshal system metrics", err)
		return
	}

	chunk := chunkData{
		fileName: EventsFileName,
		fileData: &chunkLine{chunkType: eventsChunk, line: string(line)},
	}
	fs.pushChunk(chunk)
}

func (fs *FileStream) streamFinish() {
	chunk := chunkData{Complete: &completeTrue, Exitcode: &exitcodeZero}
	fs.pushChunk(chunk)
}

func (fs *FileStream) StreamRecord(rec *service.Record) {
	fs.logger.Debug("filestream: stream record", "record", rec)
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
	chunkFileName := chunks[0].fileName // all chunks in the list should have the same file name
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
		fs.logger.CaptureFatalAndPanic("json marshal error", err)
	}

	buffer := bytes.NewBuffer(jsonData)
	req, err := retryablehttp.NewRequest(http.MethodPost, fs.path, buffer)
	if err != nil {
		fs.logger.CaptureFatalAndPanic("filestream: error creating HTTP request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := fs.httpClient.Do(req)
	if err != nil {
		fs.logger.CaptureFatalAndPanic("filestream: error making HTTP request", err)
	}
	defer func(Body io.ReadCloser) {
		if err = Body.Close(); err != nil {
			fs.logger.CaptureError("filestream: error closing response body", err)
		}
	}(resp.Body)

	var res map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		fs.logger.CaptureError("json decode error", err)
	}
	fs.pushReply(res)
	fs.logger.Debug("filestream: post response", "response", res)
}

func (fs *FileStream) Close() {
	close(fs.recordChan)
	fs.recordWait.Wait()
	close(fs.chunkChan)
	fs.chunkWait.Wait()
	close(fs.replyChan)
	fs.replyWait.Wait()
	fs.logger.Debug("filestream: closed")

}
