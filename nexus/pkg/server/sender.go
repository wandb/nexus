package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wandb/wandb/nexus/pkg/observability"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	CliVersion   = "0.0.1a1"
	MetaFilename = "wandb-metadata.json"
)

// Sender is the sender for a stream it handles the incoming messages and sends to the server
// or/and to the dispatcher/handler
type Sender struct {
	// ctx is the context for the handler
	ctx context.Context

	// logger is the logger for the sender
	logger *observability.NexusLogger

	// settings is the settings for the sender
	settings *service.Settings

	//	recordChan is the channel for outgoing messages
	recordChan chan *service.Record

	// resultChan is the channel for dispatcher messages
	resultChan chan *service.Result

	// graphqlClient is the graphql client
	graphqlClient graphql.Client

	// fileStream is the file stream
	fileStream *FileStream

	// uploader is the file uploader
	uploader *Uploader

	// RunRecord is the run record
	RunRecord *service.RunRecord
}

// NewSender creates a new Sender with the given settings
func NewSender(ctx context.Context, settings *service.Settings, logger *observability.NexusLogger) *Sender {
	sender := &Sender{
		ctx:      ctx,
		settings: settings,
		logger:   logger,
	}
	if !settings.GetXOffline().GetValue() {
		url := fmt.Sprintf("%s/graphql", settings.GetBaseUrl().GetValue())
		apiKey := settings.GetApiKey().GetValue()
		sender.graphqlClient = newGraphqlClient(url, apiKey, logger)
	}
	return sender
}

// do sending of messages to the server
func (s *Sender) do(inChan <-chan *service.Record) (<-chan *service.Result, <-chan *service.Record) {
	s.logger.Info("sender: started", "stream_id", s.settings.RunId)
	s.recordChan = make(chan *service.Record, BufferSize)
	s.resultChan = make(chan *service.Result, BufferSize)

	go func() {
		for record := range inChan {
			s.handleRecord(record)
		}
		s.logger.Info("sender: closed", "stream_id", s.settings.RunId)
	}()
	return s.resultChan, s.recordChan
}

// handleRecord sends a record
func (s *Sender) handleRecord(record *service.Record) {
	s.logger.Debug("sender: handleRecord", "record", record, "stream_id", s.settings.RunId)
	switch x := record.RecordType.(type) {
	case *service.Record_Run:
		s.handleRun(record, x.Run)
	case *service.Record_Exit:
		s.handleExit(record, x.Exit)
	case *service.Record_Alert:
		s.handleAlert(record, x.Alert)
	case *service.Record_Files:
		s.handleFiles(record, x.Files)
	case *service.Record_History:
		s.handleHistory(record, x.History)
	case *service.Record_Stats:
		s.handleStats(record, x.Stats)
	case *service.Record_OutputRaw:
		s.handleOutputRaw(record, x.OutputRaw)
	case *service.Record_Config:
		s.handleConfig(record, x.Config)
	case *service.Record_Summary:
		s.handleSummary(record, x.Summary)
	case *service.Record_Request:
		s.handleRequest(record, x.Request)
	case nil:
		err := fmt.Errorf("sender: handleRecord: nil RecordType")
		s.logger.CaptureFatalAndPanic("sender: handleRecord: nil RecordType", err)
	default:
		err := fmt.Errorf("sender: handleRecord: unexpected type %T", x)
		s.logger.CaptureFatalAndPanic("sender: handleRecord: unexpected type", err)
	}
}

// handleRequest sends a request
func (s *Sender) handleRequest(_ *service.Record, request *service.Request) {
	switch x := request.RequestType.(type) {
	case *service.Request_RunStart:
		s.handleRunStart(x.RunStart)
	case *service.Request_NetworkStatus:
		s.handleNetworkStatus(x.NetworkStatus)
	case *service.Request_Defer:
		s.handleDefer(x.Defer)
	case *service.Request_Metadata:
		s.handleMetadata(x.Metadata)
	default:
		// TODO: handle errors
	}
}

// sendResponse sends a result
func (s *Sender) sendResponse(result *service.Result) {
	s.logger.Debug("sender: sendResponse", "result", result, "stream_id", s.settings.RunId)
	s.resultChan <- result
}

// sendRecord sends a record
func (s *Sender) sendRecord(record *service.Record) {
	s.logger.Debug("sender: sendRecord", "record", record, "stream_id", s.settings.RunId)
	s.recordChan <- record
}

// handleRun starts up all the resources for a run
func (s *Sender) handleRunStart(_ *service.RunStartRequest) {
	if s.settings.GetXOffline().GetValue() {
		return
	}

	fsPath := fmt.Sprintf("%s/files/%s/%s/%s/file_stream",
		s.settings.GetBaseUrl().GetValue(), s.RunRecord.Entity, s.RunRecord.Project, s.RunRecord.RunId)
	s.fileStream = NewFileStream(fsPath, s.settings, s.logger)
	s.uploader = NewUploader(s.ctx, s.logger)
}

func (s *Sender) handleNetworkStatus(_ *service.NetworkStatusRequest) {
}

func (s *Sender) handleMetadata(request *service.MetadataRequest) {
	mo := protojson.MarshalOptions{
		Indent: "  ",
		// EmitUnpopulated: true,
	}
	jsonBytes, _ := mo.Marshal(request)
	_ = os.WriteFile(filepath.Join(s.settings.GetFilesDir().GetValue(), MetaFilename), jsonBytes, 0644)
	s.sendFile(MetaFilename)
}

func (s *Sender) handleDefer(request *service.DeferRequest) {
	switch request.State {
	case service.DeferRequest_FLUSH_FP:
		if s.uploader != nil {
			s.uploader.Close()
		}
		request.State++
		s.sendRequestDefer(request)
	case service.DeferRequest_FLUSH_FS:
		if s.fileStream != nil {
			s.fileStream.Close()
		}
		request.State++
		s.sendRequestDefer(request)
	case service.DeferRequest_END:
		close(s.recordChan)
		close(s.resultChan)
	default:
		request.State++
		s.sendRequestDefer(request)
	}
}

func (s *Sender) sendRequestDefer(request *service.DeferRequest) {
	record := &service.Record{
		RecordType: &service.Record_Request{Request: &service.Request{
			RequestType: &service.Request_Defer{Defer: request},
		}},
		Control: &service.Control{AlwaysSend: true},
	}
	s.sendRecord(record)
}

func (s *Sender) parseConfigUpdate(config *service.ConfigRecord) map[string]interface{} {
	datas := make(map[string]interface{})

	// TODO: handle deletes and nested key updates
	for _, d := range config.GetUpdate() {
		j := d.GetValueJson()
		var data interface{}
		if err := json.Unmarshal([]byte(j), &data); err != nil {
			s.logger.CaptureFatalAndPanic("unmarshal problem", err)
		}
		datas[d.GetKey()] = data
	}
	return datas
}

func (s *Sender) updateConfigTelemetry(config map[string]interface{}) {
	got := config["_wandb"]
	switch v := got.(type) {
	case map[string]interface{}:
		v["cli_version"] = CliVersion
	default:
		err := fmt.Errorf("can not parse config _wandb, saw: %v", v)
		s.logger.CaptureFatalAndPanic("sender received error", err)
	}
}

func (s *Sender) getValueConfig(config map[string]interface{}) map[string]map[string]interface{} {

	datas := make(map[string]map[string]interface{})
	for key, elem := range config {
		datas[key] = make(map[string]interface{})
		datas[key]["value"] = elem
	}
	return datas
}

func (s *Sender) handleRun(record *service.Record, run *service.RunRecord) {

	var ok bool
	s.RunRecord, ok = proto.Clone(run).(*service.RunRecord)
	if !ok {
		err := fmt.Errorf("sender: handleRun: failed to clone RunRecord")
		s.logger.CaptureFatalAndPanic("sender received error", err)
	}

	config := s.handleConfig(record, run.Config)

	if s.graphqlClient != nil {

		var tags []string
		data, err := UpsertBucket(
			s.ctx,           // ctx
			s.graphqlClient, // client
			nil,             // id
			&run.RunId,      // name
			nil,             // project
			nil,             // entity
			nil,             // groupName
			nil,             // description
			nil,             // displayName
			nil,             // notes
			nil,             // commit
			&config,         // config
			nil,             // host
			nil,             // debug
			nil,             // program
			nil,             // repo
			nil,             // jobType
			nil,             // state
			nil,             // sweep
			tags,            // tags []string,
			nil,             // summaryMetrics
		)
		if err != nil {
			err = fmt.Errorf("sender: handleRun: failed to upsert bucket: %s", err)
			s.logger.CaptureFatalAndPanic("sender received error", err)
		}

		s.RunRecord.DisplayName = *data.UpsertBucket.Bucket.DisplayName
		s.RunRecord.Project = data.UpsertBucket.Bucket.Project.Name
		s.RunRecord.Entity = data.UpsertBucket.Bucket.Project.Entity.Name

		// TODO: remove this
		s.settings.Project = &wrapperspb.StringValue{Value: data.UpsertBucket.Bucket.Project.Name}
		s.settings.Entity = &wrapperspb.StringValue{Value: data.UpsertBucket.Bucket.Project.Entity.Name}
	}

	result := &service.Result{
		ResultType: &service.Result_RunResult{
			RunResult: &service.RunUpdateResult{Run: s.RunRecord},
		},
		Control: record.Control,
		Uuid:    record.Uuid,
	}
	s.sendResponse(result)
}

// handleHistory sends a history record to the file stream,
// which will then send it to the server
func (s *Sender) handleHistory(record *service.Record, _ *service.HistoryRecord) {
	if s.fileStream != nil {
		s.fileStream.StreamRecord(record)
	}
}

func (s *Sender) handleStats(record *service.Record, _ *service.StatsRecord) {
	if s.fileStream != nil {
		s.fileStream.StreamRecord(record)
	}
}

func (s *Sender) handleOutputRaw(record *service.Record, outputRaw *service.OutputRawRecord) {
	// TODO: match logic handling of lines to the one in the python version
	// - handle carriage returns (for tqdm-like progress bars)
	// - handle caching multiple (non-new lines) and sending them in one chunk
	// - handle lines longer than ~60_000 characters

	if s.fileStream == nil {
		return
	}

	// ignore empty "new lines"
	if outputRaw.Line == "\n" {
		return
	}
	t := time.Now().UTC().Format(time.RFC3339)
	outputRaw.Line = fmt.Sprintf("%s %s", t, outputRaw.Line)
	if outputRaw.OutputType == service.OutputRawRecord_STDERR {
		outputRaw.Line = fmt.Sprintf("ERROR %s", outputRaw.Line)
	}
	s.fileStream.StreamRecord(record)
}

func (s *Sender) handleConfig(_ *service.Record, config *service.ConfigRecord) string {
	var configString string
	if config != nil {
		config := s.parseConfigUpdate(config)
		s.updateConfigTelemetry(config)
		valueConfig := s.getValueConfig(config)
		configJson, err := json.Marshal(valueConfig)
		if err != nil {
			err = fmt.Errorf("sender: handleConfig: failed to marshal config: %s", err)
			s.logger.CaptureError("sender received error", err)
			return "{}"
		}
		configString = string(configJson)
	} else {
		configString = "{}"
	}
	return configString
}

func (s *Sender) handleSummary(_ *service.Record, _ *service.SummaryRecord) {

}

func (s *Sender) handleAlert(_ *service.Record, alert *service.AlertRecord) {
	if s.graphqlClient == nil {
		return
	}

	// TODO: handle invalid alert levels
	severity := AlertSeverity(alert.Level)
	waitDuration := time.Duration(alert.WaitDuration) * time.Second

	data, err := NotifyScriptableRunAlert(
		s.ctx,
		s.graphqlClient,
		s.RunRecord.Entity,
		s.RunRecord.Project,
		s.RunRecord.RunId,
		alert.Title,
		alert.Text,
		&severity,
		&waitDuration,
	)
	if err != nil {
		err = fmt.Errorf("sender: handleAlert: failed to notify scriptable run alert: %s", err)
		s.logger.CaptureError("sender received error", err)
	} else {
		s.logger.Info("sender: handleAlert: notified scriptable run alert", "data", data)
	}

}

// handleExit sends an exit record to the server and triggers the shutdown of the stream
func (s *Sender) handleExit(record *service.Record, _ *service.RunExitRecord) {
	if s.fileStream != nil {
		s.fileStream.StreamRecord(record)
	}
	result := &service.Result{
		ResultType: &service.Result_ExitResult{ExitResult: &service.RunExitResult{}},
		Control:    record.Control,
		Uuid:       record.Uuid,
	}
	s.sendResponse(result)

	request := &service.Request{RequestType: &service.Request_Defer{
		Defer: &service.DeferRequest{State: service.DeferRequest_BEGIN}},
	}
	rec := &service.Record{
		RecordType: &service.Record_Request{Request: request},
		Control:    record.Control,
		Uuid:       record.Uuid,
		XInfo:      record.XInfo,
	}
	s.sendRecord(rec)
}

// handleFiles iterates over the files in the FilesRecord and sends them to
func (s *Sender) handleFiles(_ *service.Record, filesRecord *service.FilesRecord) {
	files := filesRecord.GetFiles()
	for _, file := range files {
		s.sendFile(file.GetPath())
	}
}

// sendFile sends a file to the server
func (s *Sender) sendFile(name string) {
	if s.graphqlClient == nil || s.uploader == nil {
		return
	}

	if s.RunRecord == nil {
		err := fmt.Errorf("sender: sendFile: RunRecord not set")
		s.logger.CaptureFatalAndPanic("sender received error", err)
	}

	data, err := RunUploadUrls(
		s.ctx,
		s.graphqlClient,
		s.RunRecord.Project,
		[]*string{&name},
		&s.RunRecord.Entity,
		s.RunRecord.RunId,
		nil,
	)
	if err != nil {
		err = fmt.Errorf("sender: sendFile: failed to get upload urls: %s", err)
		s.logger.CaptureFatalAndPanic("sender received error", err)
	}

	fullPath := filepath.Join(s.settings.GetFilesDir().GetValue(), name)
	edges := data.GetModel().GetBucket().GetFiles().GetEdges()
	for _, e := range edges {
		task := &UploadTask{fullPath, *e.GetNode().GetUrl()}
		s.uploader.AddTask(task)
	}
}
