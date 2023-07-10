package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"encoding/json"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"golang.org/x/exp/slog"
)

const CliVersion string = "0.0.1a1"
const MetaFilename string = "wandb-metadata.json"

type Sender struct {
	ctx            context.Context
	settings       *service.Settings
	inChan         chan *service.Record
	outChan        chan<- *service.Record
	dispatcherChan dispatchChannel
	graphqlClient  graphql.Client
	fileStream     *FileStream
	uploader       *Uploader
	// todo: add watcher that would use https://pkg.go.dev/github.com/fsnotify/fsnotify
	run    *service.RunRecord
	logger *slog.Logger
}

// NewSender creates a new Sender instance
func NewSender(ctx context.Context, settings *service.Settings, logger *slog.Logger) *Sender {
	sender := &Sender{
		ctx:      ctx,
		settings: settings,
		inChan:   make(chan *service.Record),
		logger:   logger,
	}
	sender.logger.Debug("Sender: dispatch")
	url := fmt.Sprintf("%s/graphql", settings.GetBaseUrl().GetValue())
	sender.graphqlClient = newGraphqlClient(url, settings.GetApiKey().GetValue())
	return sender
}

// do starts the sender
func (s *Sender) do() error {
	s.logger.Debug("starting sender")
	for msg := range s.inChan {
		s.logger.Debug("sending record", slog.String("record", msg.String()))
		if err := s.sendRecord(msg); err != nil {
			return err
		}
	}
	s.logger.Debug("closed")
	return nil
}

// sendRecord sends a record
func (s *Sender) sendRecord(msg *service.Record) error {
	switch x := msg.RecordType.(type) {
	case *service.Record_Run:
		return s.sendRun(msg, x.Run)
	case *service.Record_Exit:
		return s.sendExit(msg, x.Exit)
	case *service.Record_Files:
		s.sendFiles(msg, x.Files)
	case *service.Record_History:
		s.sendHistory(msg, x.History)
	case *service.Record_Request:
		s.sendRequest(msg, x.Request)
	case nil:
		return errors.New("record type is nil")
	default:
		return errors.New("unknown record type")
	}
	return nil
}

func (s *Sender) sendRequest(_ *service.Record, req *service.Request) {
	switch x := req.RequestType.(type) {
	case *service.Request_RunStart:
		s.sendRunStart(x.RunStart)
	case *service.Request_NetworkStatus:
		s.sendNetworkStatusRequest(x.NetworkStatus)
	case *service.Request_Defer:
		s.sendDefer(x.Defer)
	case *service.Request_Metadata:
		s.sendMetadata(x.Metadata)
	default:
	}
}

func (s *Sender) sendRunStart(_ *service.RunStartRequest) {
	fsPath := fmt.Sprintf("%s/files/%s/%s/%s/file_stream",
		s.settings.GetBaseUrl().GetValue(), s.run.Entity, s.run.Project, s.run.RunId)
	s.fileStream = NewFileStream(fsPath, s.settings, s.logger)
	s.uploader = NewUploader(s.ctx, s.logger)
}

func (s *Sender) sendNetworkStatusRequest(_ *service.NetworkStatusRequest) {
}

func (s *Sender) sendMetadata(req *service.MetadataRequest) {
	mo := protojson.MarshalOptions{
		Indent: "  ",
		// EmitUnpopulated: true,
	}
	jsonBytes, _ := mo.Marshal(req)
	_ = os.WriteFile(filepath.Join(s.settings.GetFilesDir().GetValue(), MetaFilename), jsonBytes, 0644)
	s.sendFile(MetaFilename)
}

func (s *Sender) sendDefer(req *service.DeferRequest) {
	switch req.State {
	case service.DeferRequest_FLUSH_FP:
		s.uploader.close()
		req.State++
		s.sendRequestDefer(req)
	case service.DeferRequest_FLUSH_FS:
		s.fileStream.close()
		req.State++
		s.sendRequestDefer(req)
	case service.DeferRequest_END:
		close(s.outChan)
	default:
		req.State++
		s.sendRequestDefer(req)
	}
}

func (s *Sender) sendRequestDefer(req *service.DeferRequest) {
	r := service.Record{
		RecordType: &service.Record_Request{Request: &service.Request{
			RequestType: &service.Request_Defer{Defer: req},
		}},
		Control: &service.Control{AlwaysSend: true},
	}
	s.outChan <- &r
}

func (s *Sender) parseConfigUpdate(config *service.ConfigRecord) map[string]interface{} {
	datas := make(map[string]interface{})

	// TODO: handle deletes and nested key updates
	for _, d := range config.GetUpdate() {
		j := d.GetValueJson()
		var data interface{}
		err := json.Unmarshal([]byte(j), &data)
		if err != nil {
			LogFatalError(s.logger, "unmarshal problem", err)
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
		LogFatal(s.logger, fmt.Sprintf("can not parse config _wandb, saw: %v", v))
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

func (s *Sender) sendRun(msg *service.Record, record *service.RunRecord) error {

	run, ok := proto.Clone(record).(*service.RunRecord)
	if !ok {
		return errors.New("failed to clone run record")
	}

	config := s.parseConfigUpdate(record.Config)
	s.updateConfigTelemetry(config)
	valueConfig := s.getValueConfig(config)
	configJson, err := json.Marshal(valueConfig)
	if err != nil {
		return err
	}
	configString := string(configJson)

	var tags []string
	resp, err := UpsertBucket(
		s.ctx,
		s.graphqlClient,
		nil,           // id
		&record.RunId, // name
		nil,           // project
		nil,           // entity
		nil,           // groupName
		nil,           // description
		nil,           // displayName
		nil,           // notes
		nil,           // commit
		&configString, // config
		nil,           // host
		nil,           // debug
		nil,           // program
		nil,           // repo
		nil,           // jobType
		nil,           // state
		nil,           // sweep
		tags,          // tags []string,
		nil,           // summaryMetrics
	)
	if err != nil {
		return err
	}

	run.DisplayName = *resp.UpsertBucket.Bucket.DisplayName
	run.Project = resp.UpsertBucket.Bucket.Project.Name
	run.Entity = resp.UpsertBucket.Bucket.Project.Entity.Name

	s.run = run
	result := &service.Result{
		ResultType: &service.Result_RunResult{RunResult: &service.RunUpdateResult{Run: run}},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	s.dispatcherChan.Deliver(result)
	return nil
}

func (s *Sender) sendHistory(msg *service.Record, _ *service.HistoryRecord) {
	if s.fileStream != nil {
		s.fileStream.stream(msg)
	}
}

func (s *Sender) sendExit(msg *service.Record, _ *service.RunExitRecord) error {
	// do exit via filestream
	s.fileStream.stream(msg)

	result := &service.Result{
		ResultType: &service.Result_ExitResult{ExitResult: &service.RunExitResult{}},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	s.dispatcherChan.Deliver(result)
	//RequestType: &service.Request_Defer{Defer: &service.DeferRequest{State: service.DeferRequest_BEGIN}}
	req := &service.Request{RequestType: &service.Request_Defer{Defer: &service.DeferRequest{State: service.DeferRequest_BEGIN}}}
	rec := &service.Record{RecordType: &service.Record_Request{Request: req}, Control: msg.Control, Uuid: msg.Uuid}
	return s.sendRecord(rec)
}

func (s *Sender) sendFiles(_ *service.Record, filesRecord *service.FilesRecord) {
	files := filesRecord.GetFiles()
	for _, file := range files {
		s.sendFile(file.GetPath())
	}
}

func (s *Sender) sendFile(path string) {

	if s.run == nil {
		err := errors.New("upsert run not called before do file")
		s.logger.Error(err.Error())
		panic(err)
	}

	entity := s.run.Entity
	resp, err := RunUploadUrls(
		s.ctx,
		s.graphqlClient,
		s.run.Project,
		[]*string{&path},
		&entity,
		s.run.RunId,
		nil, // description
	)
	if err != nil {
		s.logger.Error("error getting upload urls", slog.String("error", err.Error()))
	}

	fullPath := filepath.Join(s.settings.GetFilesDir().GetValue(), path)
	edges := resp.GetModel().GetBucket().GetFiles().GetEdges()
	for _, e := range edges {
		task := &UploadTask{fullPath, *e.GetNode().GetUrl()}
		s.logger.Debug("sending file", task)
		s.uploader.addTask(task)
	}
}
