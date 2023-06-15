package server

import (
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
	settings      *Settings
	inChan        chan *service.Record
	graphqlClient graphql.Client
	fstream       *FileStream
	run           *service.RunRecord
	// handler        *Handler
	deferResult    *service.Result
	dispatcherChan dispatchChannel
}

func NewSender(settings *Settings, dispatcherChan dispatchChannel) *Sender {
	sender := &Sender{
		settings:       settings,
		inChan:         make(chan *service.Record),
		dispatcherChan: dispatcherChan,
	}
	return sender
}

func (s *Sender) start() {
	log.Debug("SENDER: OPEN")
	s.senderInit()
	for msg := range s.inChan {
		log.Debug("SENDER *******")
		log.WithFields(log.Fields{"record": msg}).Debug("SENDER: got msg")
		s.networkSendRecord(msg)
		// handleLogWriter(s, msg)
	}
	log.Debug("SENDER: FIN")
}

func (s *Sender) Deliver(msg *service.Record) {
	s.inChan <- msg
}

func (s *Sender) startRunWorkers() {
	fsPath := fmt.Sprintf("%s/files/%s/%s/%s/file_stream",
		s.settings.BaseURL, s.run.Entity, s.run.Project, s.run.RunId)
	s.fstream = NewFileStream(fsPath, s.settings)
}

// func (s *Sender) SetHandler(h *Handler) {
//	s.handler = h
// }

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

func (s *Sender) sendNetworkStatusRequest(_ *service.NetworkStatusRequest) {
}

func (s *Sender) sendDefer(req *service.DeferRequest) {
	log.Debug("SENDER: DEFER STOP")
	// fmt.Println("DEFER", req)
	// fmt.Println("DEFER2", req.State)
	done := false
	switch req.State {
	case service.DeferRequest_FLUSH_FS:
		s.fstream.flush()
	case service.DeferRequest_END:
		done = true
	default:
	}
	if done {
		s.dispatcherChan.Deliver(s.deferResult)
	} else {
		req.State += 1
		s.doSendDefer(req)
	}
}

func (s *Sender) sendRunStart(_ *service.RunStartRequest) {
	s.startRunWorkers()
}

func (s *Sender) sendHistory(msg *service.Record, _ *service.HistoryRecord) {
	if s.fstream != nil {
		s.fstream.StreamRecord(msg)
	}
}

func (s *Sender) sendRequest(_ *service.Record, req *service.Request) {
	switch x := req.RequestType.(type) {
	case *service.Request_NetworkStatus:
		s.sendNetworkStatusRequest(x.NetworkStatus)
	case *service.Request_RunStart:
		s.sendRunStart(x.RunStart)
	case *service.Request_Defer:
		s.sendDefer(x.Defer)
	default:
	}
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
		panic("bad2rec")
	default:
		bad := fmt.Sprintf("REC UNKNOWN type %T", x)
		panic(bad)
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
	// req.Header.Set("Content-Type", "application/octet-stream")
	rsp, err := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		log.Printf("Request failed with response code: %d", rsp.StatusCode)
		panic("badness")
	}
	// fmt.Println("GOTERR", rsp, err)
	if err != nil {
		return err
	}
	return nil
}

func (s *Sender) sendFiles(msg *service.Record, filesRecord *service.FilesRecord) {
	myfiles := filesRecord.GetFiles()
	for _, fi := range myfiles {
		s.doSendFile(msg, fi)
	}
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
	// func RunUploadUrls(
	//     ctx context.Context,
	//     client graphql.Client,
	//     name string,
	//     files []*string,
	//     entity *string,
	//     run string,
	//     description *string,
	// ) (*RunUploadUrlsResponse, error) {

	// got := proto.MarshalTextString(resp)
	// got := protojson.Format(resp)
	// fmt.Printf("got: %s\n", got)
	model := resp.GetModel()
	bucket := model.GetBucket()

	// runID := bucket.GetId()
	// fmt.Printf("ID: %s\n", runID)

	fileList := bucket.GetFiles()

	// headers := fileList.GetUploadHeaders()
	// fmt.Printf("HEADS: %s\n", headers)

	edges := fileList.GetEdges()
	// result := make([]*RunUploadUrlsModelProjectBucketRunFilesFileConnectionEdgesFileEdgeNodeFile, len(edges))
	result := make([]*string, len(edges))
	for i, e := range edges {
		node := e.GetNode()
		// name := node.GetName()
		url := node.GetUrl()
		// updated := node.GetUpdatedAt()
		result[i] = url
		// fmt.Printf("url: %d %s %s %s\n", i, *url, name, updated)
		err = sendData(fname, *url)
		if err != nil {
			log.Error("error sending data", err)
		}
	}
	// fmt.Printf("got: %s\n", result)
}

func (s *Sender) doSendDefer(deferRequest *service.DeferRequest) {
	req := service.Request{
		RequestType: &service.Request_Defer{Defer: deferRequest},
	}
	r := service.Record{
		RecordType: &service.Record_Request{Request: &req},
		Control:    &service.Control{AlwaysSend: true},
	}
	fmt.Printf("this is %s", &r)
	// todo: fix this logic
	// s.handler.Handle(&r)
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
	// FIXME: hack to make sure that filestream has no data before continuing
	// time.Sleep(2 * time.Second)
	// h.shutdownStream()
	s.deferResult = result
	deferRequest := &service.DeferRequest{}
	s.doSendDefer(deferRequest)
}

func (s *Sender) sendRun(msg *service.Record, record *service.RunRecord) {

	keepRun, ok := proto.Clone(record).(*service.RunRecord)
	if !ok {
		log.Fatal("error")
	}

	// fmt.Println("SEND", record)
	ctx := context.Background()
	// resp, err := Viewer(ctx, s.graphqlClient)
	// fmt.Println(resp, err)
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

	// fmt.Println("RESP::", keepRun)

	// (*UpsertBucketResponse, error) {
	/*
		id *string,
		name *string,
		project *string,
		entity *string,
		groupName *string,
		description *string,
		displayName *string,
		notes *string,
		commit *string,
		config *string,
		host *string,
		debug *bool,
		program *string,
		repo *string,
		jobType *string,
		state *string,
		sweep *string,
		tags []string,
		summaryMetrics *string,
	*/

	runResult := &service.RunUpdateResult{Run: keepRun}
	result := &service.Result{
		ResultType: &service.Result_RunResult{RunResult: runResult},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	log.Debug("sending run result ", result)
	s.dispatcherChan.Deliver(result)
}
