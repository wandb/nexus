package server

import (
	"time"

	"bytes"
	"os"

	"context"
	"fmt"

	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"

	log "github.com/sirupsen/logrus"
)

type Sender struct {
	settings       *Settings
	inChan         chan *service.Record
	outChan        chan<- *service.Record
	dispatcherChan dispatchChannel
	graphqlClient  graphql.Client
	fileStream     *FileStream
	run            *service.RunRecord
}

// NewSender creates a new Sender instance
func NewSender(ctx context.Context, settings *Settings) *Sender {
	sender := &Sender{
		settings: settings,
		inChan:   make(chan *service.Record),
	}
	log.Debug("Sender: start")
	url := fmt.Sprintf("%s/graphql", settings.BaseURL)
	sender.graphqlClient = newGraphqlClient(url, settings.ApiKey)
	return sender
}

// close closes the sender's resources
func (s *Sender) close() {
	log.Debug("Sender: close")
	s.fileStream.close()
}

// sendRecord sends a record
func (s *Sender) sendRecord(msg *service.Record) {
	switch x := msg.RecordType.(type) {
	case *service.Record_Run:
		s.sendRun(msg, x.Run)
	case *service.Record_Exit:
		s.sendRunExit(msg, x.Exit)
	case *service.Record_Files:
		s.sendFiles(msg, x.Files)
	case *service.Record_History:
		log.Debug("Sender: sendRecord", msg)
		s.sendHistory(msg, x.History)
	case *service.Record_Request:
		s.sendRequest(msg, x.Request)
	case nil:
		log.Fatal("sender: sendRecord: nil RecordType")
	default:
		log.Fatalf("sender: sendRecord: unexpected type %T", x)
	}
}

func (s *Sender) sendRequest(_ *service.Record, req *service.Request) {
	switch x := req.RequestType.(type) {
	case *service.Request_RunStart:
		s.sendRunStart(x.RunStart)
	case *service.Request_NetworkStatus:
		s.sendNetworkStatusRequest(x.NetworkStatus)
	case *service.Request_Defer:
		s.sendDefer(x.Defer)
	default:
	}
}

func (s *Sender) sendRunStart(_ *service.RunStartRequest) {
	fsPath := fmt.Sprintf("%s/files/%s/%s/%s/file_stream",
		s.settings.BaseURL, s.run.Entity, s.run.Project, s.run.RunId)
	s.fileStream = NewFileStream(fsPath, s.settings)
	log.Debug("Sender: sendRunStart: start file stream")
	go s.fileStream.start()
}

func (s *Sender) sendNetworkStatusRequest(_ *service.NetworkStatusRequest) {
}

func (s *Sender) sendDefer(req *service.DeferRequest) {
	s.dispatcherChan.Deliver(nil)
}

func (s *Sender) sendRun(msg *service.Record, record *service.RunRecord) {

	run, ok := proto.Clone(record).(*service.RunRecord)
	if !ok {
		log.Fatal("error")
	}

	ctx := context.Background()
	var tags []string
	resp, err := UpsertBucket(
		ctx, s.graphqlClient,
		nil,           // id
		&record.RunId, // name
		nil,           // project
		nil,           // entity
		nil,           // groupName
		nil,           // description
		nil,           // displayName
		nil,           // notes
		nil,           // commit
		nil,           // config
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
		log.Error("error upserting bucket", err)
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
	log.Debug("sending run result ", result)
	s.dispatcherChan.Deliver(result)
}

func (s *Sender) sendHistory(msg *service.Record, _ *service.HistoryRecord) {
	log.Debug("sending history result ", msg)
	if s.fileStream != nil {
		s.fileStream.stream(msg)
	}
}

func (s *Sender) sendRunExit(msg *service.Record, _ *service.RunExitRecord) {
	// send exit via filestream
	log.Debug("sending run exit result ", msg)
	s.fileStream.stream(msg)

	result := &service.Result{
		ResultType: &service.Result_ExitResult{ExitResult: &service.RunExitResult{}},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	s.dispatcherChan.Deliver(result)
}

func (s *Sender) sendFiles(msg *service.Record, filesRecord *service.FilesRecord) {
	files := filesRecord.GetFiles()
	for _, file := range files {
		s.sendFile(msg, file)
	}
}

func sendData(fileName, urlPath string) error {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	b, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, urlPath, bytes.NewReader(b))
	if err != nil {
		return err
	}
	rsp, err := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		log.Fatalf("Request failed with response code: %d", rsp.StatusCode)
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Sender) sendFile(_ *service.Record, fileItem *service.FilesItem) {

	if s.run == nil {
		log.Fatal("upsert run not called before send db")
	}

	entity := s.run.Entity
	path := fileItem.GetPath()
	resp, err := RunUploadUrls(
		context.Background(),
		s.graphqlClient,
		s.run.Project,
		[]*string{&path},
		&entity,
		s.run.RunId,
		nil, // description
	)
	if err != nil {
		log.Error("error getting upload urls", err)
	}

	edges := resp.GetModel().GetBucket().GetFiles().GetEdges()
	for _, e := range edges {
		url := e.GetNode().GetUrl()
		if err = sendData(path, *url); err != nil {
			log.Error("error sending data", err)
		}
	}
}
