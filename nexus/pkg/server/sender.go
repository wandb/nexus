package server

import (
	"sync"
	"time"

	"bytes"
	"os"

	"context"
	"encoding/base64"
	"fmt"

	// "google.golang.org/protobuf/proto"
	// "google.golang.org/protobuf/encoding/protojson"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"

	log "github.com/sirupsen/logrus"
)

type Sender struct {
	task           *Task
	settings       *Settings
	inChan         chan *service.Record
	graphqlClient  graphql.Client
	fstream        *FileStream
	run            *service.RunRecord
	deferResult    *service.Result
	dispatcherChan dispatchChannel
}

func NewSender(ctx context.Context, settings *Settings, dispatcherChan dispatchChannel) *Sender {
	task := NewTask(ctx)
	sender := &Sender{
		task:           task,
		settings:       settings,
		inChan:         make(chan *service.Record),
		dispatcherChan: dispatcherChan,
	}
	return sender
}

func (s *Sender) senderInit() {
	httpClient := http.Client{
		Transport: &authedTransport{
			key:     s.settings.ApiKey,
			wrapped: http.DefaultTransport,
		},
	}
	url := fmt.Sprintf("%s/graphql", s.settings.BaseURL)
	s.graphqlClient = graphql.NewClient(url, &httpClient)
}

func (s *Sender) start() {
	loopWg := &sync.WaitGroup{}
	loopWg.Add(1)

	defer func() {
		s.close()
		loopWg.Wait()
		s.task.wg.Done()
	}()

	go func() {
		log.Debug("SENDER: OPEN")
		s.senderInit()
		for msg := range s.inChan {
			log.Debug("SENDER *******")
			log.WithFields(log.Fields{"record": msg}).Debug("SENDER: got msg")
			s.networkSendRecord(msg)
		}
		loopWg.Done()
		log.Debug("SENDER: FIN")
	}()

	<-s.task.ctx.Done()
}

func (s *Sender) close() {
	log.Debug("SENDER: CLOSE")
	close(s.inChan)
	s.fstream.flush()
}

func (s *Sender) Deliver(msg *service.Record) {
	s.inChan <- msg
}

type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Basic "+basicAuth("api", t.key))
	req.Header.Set("User-Agent", "wandb-nexus")
	// req.Header.Set("X-WANDB-USERNAME", "jeff")
	// req.Header.Set("X-WANDB-USER-EMAIL", "jeff@wandb.com")
	return t.wrapped.RoundTrip(req)
}

func (s *Sender) networkSendRecord(msg *service.Record) {
	switch x := msg.RecordType.(type) {
	case *service.Record_Run:
		s.sendRun(msg, x.Run)
	case *service.Record_Exit:
		s.sendRunExit(msg, x.Exit)
	case *service.Record_Files:
		s.sendFiles(msg, x.Files)
	case *service.Record_History:
		s.sendHistory(msg, x.History)
	case *service.Record_Request:
		s.sendRequest(msg, x.Request)
	case nil:
		// The field is not set.
		panic("sender: networkSendRecord: nil RecordType")
	default:
		bad := fmt.Sprintf("sender: networkSendRecord: unexpected type %T", x)
		panic(bad)
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
	s.fstream = NewFileStream(fsPath, s.settings)
}

func (s *Sender) sendNetworkStatusRequest(_ *service.NetworkStatusRequest) {
}

func (s *Sender) sendDefer(req *service.DeferRequest) {
	switch req.State {
	default:
		s.dispatcherChan.Deliver(s.deferResult)
	}
}

func (s *Sender) sendRun(msg *service.Record, record *service.RunRecord) {

	keepRun, ok := proto.Clone(record).(*service.RunRecord)
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

	displayName := *resp.UpsertBucket.Bucket.DisplayName
	projectName := resp.UpsertBucket.Bucket.Project.Name
	entityName := resp.UpsertBucket.Bucket.Project.Entity.Name
	keepRun.DisplayName = displayName
	keepRun.Project = projectName
	keepRun.Entity = entityName

	s.run = keepRun
	runResult := &service.RunUpdateResult{Run: keepRun}
	result := &service.Result{
		ResultType: &service.Result_RunResult{RunResult: runResult},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	log.Debug("sending run result ", result)
	s.dispatcherChan.Deliver(result)
}

func (s *Sender) sendHistory(msg *service.Record, _ *service.HistoryRecord) {
	if s.fstream != nil {
		s.fstream.StreamRecord(msg)
	}
}

func (s *Sender) sendRunExit(msg *service.Record, _ *service.RunExitRecord) {
	// send exit via filestream
	s.fstream.StreamRecord(msg)

	// TODO: need to flush stuff before responding with exit
	runExitResult := &service.RunExitResult{}
	result := &service.Result{
		ResultType: &service.Result_ExitResult{ExitResult: runExitResult},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	s.dispatcherChan.Deliver(result)
}

func (s *Sender) sendFiles(msg *service.Record, filesRecord *service.FilesRecord) {
	files := filesRecord.GetFiles()
	for _, fi := range files {
		s.doSendFile(msg, fi)
	}
}

func sendData(fname, urlPath string) error {
	method := "PUT"
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	b, err := os.ReadFile(fname)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, urlPath, bytes.NewReader(b))
	if err != nil {
		return err
	}
	rsp, err := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		log.Printf("Request failed with response code: %d", rsp.StatusCode)
		panic("badness")
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Sender) doSendFile(_ *service.Record, fileItem *service.FilesItem) {
	// fmt.Println("GOTFILE", filesRecord)
	path := fileItem.GetPath()

	if s.run == nil {
		panic("upsert run not called before send db")
	}

	ctx := context.Background()
	project := s.run.Project
	runId := s.run.RunId
	entity := s.run.Entity
	fname := path
	files := []*string{&fname}

	resp, err := RunUploadUrls(
		ctx,
		s.graphqlClient,
		project,
		files,
		&entity,
		runId,
		nil, // description
	)
	if err != nil {
		log.Error("error getting upload urls", err)
	}

	model := resp.GetModel()
	bucket := model.GetBucket()
	fileList := bucket.GetFiles()
	edges := fileList.GetEdges()
	result := make([]*string, len(edges))
	for i, e := range edges {
		node := e.GetNode()
		url := node.GetUrl()
		result[i] = url
		err = sendData(fname, *url)
		if err != nil {
			log.Error("error sending data", err)
		}
	}
}
