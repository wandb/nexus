package server

import (
	// "flag"
	// "io"
	// "google.golang.org/protobuf/reflect/protoreflect"
	"sync"
	"time"

	"bytes"
	"io/ioutil"

	"context"
	"encoding/base64"
	"fmt"
	// "google.golang.org/protobuf/proto"
	// "google.golang.org/protobuf/encoding/protojson"
	"github.com/Khan/genqlient/graphql"
	"github.com/wandb/wandb/nexus/service"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type Sender struct {
	senderChan    chan *service.Record
	wg            *sync.WaitGroup
	graphqlClient graphql.Client
	respondResult func(result *service.Result)
	settings      *Settings
	run           *service.RunRecord
}

func NewSender(wg *sync.WaitGroup, respondResult func(result *service.Result), settings *Settings) *Sender {
	sender := Sender{
		senderChan:    make(chan *service.Record),
		wg:            wg,
		respondResult: respondResult,
		settings:      settings,
	}

	sender.wg.Add(1)
	go sender.senderGo()
	return &sender
}

func (sender *Sender) Stop() {
	close(sender.senderChan)
}

func (sender *Sender) SendRecord(rec *service.Record) {
	if sender.settings.Offline {
		return
	}
	sender.senderChan <- rec
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

func (sender *Sender) senderInit() {
	httpClient := http.Client{
		Transport: &authedTransport{
			key:     sender.settings.ApiKey,
			wrapped: http.DefaultTransport,
		},
	}
	url := fmt.Sprintf("%s/graphql", sender.settings.BaseURL)
	sender.graphqlClient = graphql.NewClient(url, &httpClient)
}

func (sender *Sender) sendNetworkStatusRequest(msg *service.NetworkStatusRequest) {
}

func (sender *Sender) sendRequest(msg *service.Record, req *service.Request) {
	switch x := req.RequestType.(type) {
	case *service.Request_NetworkStatus:
		sender.sendNetworkStatusRequest(x.NetworkStatus)
	default:
	}
}

func (sender *Sender) networkSendRecord(msg *service.Record) {
	switch x := msg.RecordType.(type) {
	case *service.Record_Run:
		// fmt.Println("rungot:", x)
		sender.networkSendRun(msg, x.Run)
	case *service.Record_Files:
		sender.networkSendFile(msg, x.Files)
	case *service.Record_Request:
		sender.sendRequest(msg, x.Request)
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

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, urlPath, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	rsp, _ := client.Do(req)
	if rsp.StatusCode != http.StatusOK {
		log.Printf("Request failed with response code: %d", rsp.StatusCode)
	}
	return nil
}

func (sender *Sender) networkSendFile(msg *service.Record, filesRecord *service.FilesRecord) {
	fmt.Println("GOTFILE", filesRecord)
	fileList = filesRecord.GetFiles()
	if len(fileList) != 0 {
		panic("unsupported len")
	}
	path := fileList[0].GetPath()

	if sender.run == nil {
		panic("upsert run not called before send file")
	}

	ctx := context.Background()
	project := sender.run.Project
	runId := sender.run.RunId
	entity := sender.run.Entity
	fname := path
	files := []*string{&fname}

	resp, err := RunUploadUrls(
		ctx,
		sender.graphqlClient,
		project,
		files,
		&entity,
		runId,
		nil, // description
	)
	check(err)
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

	runID := bucket.GetId()
	fmt.Printf("ID: %s\n", runID)

	fileList := bucket.GetFiles()

	headers := fileList.GetUploadHeaders()
	fmt.Printf("HEADS: %s\n", headers)

	edges := fileList.GetEdges()
	// result := make([]*RunUploadUrlsModelProjectBucketRunFilesFileConnectionEdgesFileEdgeNodeFile, len(edges))
	result := make([]*string, len(edges))
	for i, e := range edges {
		node := e.GetNode()
		name := node.GetName()
		url := node.GetUrl()
		updated := node.GetUpdatedAt()
		result[i] = url
		fmt.Printf("url: %d %s %s %s\n", i, *url, name, updated)
		err = sendData(fname, *url)
		check(err)
	}
	// fmt.Printf("got: %s\n", result)
}

func (sender *Sender) networkSendRun(msg *service.Record, record *service.RunRecord) {

	keepRun := *record

	// fmt.Println("SEND", record)
	ctx := context.Background()
	// resp, err := Viewer(ctx, sender.graphqlClient)
	// fmt.Println(resp, err)
	tags := []string{}
	resp, err := UpsertBucket(
		ctx, sender.graphqlClient,
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
	check(err)

	displayName := *resp.UpsertBucket.Bucket.DisplayName
	projectName := resp.UpsertBucket.Bucket.Project.Name
	entityName := resp.UpsertBucket.Bucket.Project.Entity.Name
	keepRun.DisplayName = displayName
	keepRun.Project = projectName
	keepRun.Entity = entityName

	sender.run = &keepRun

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

	runResult := &service.RunUpdateResult{Run: &keepRun}
	result := &service.Result{
		ResultType: &service.Result_RunResult{runResult},
		Control:    msg.Control,
		Uuid:       msg.Uuid,
	}
	sender.respondResult(result)
}

func (sender *Sender) senderGo() {
	defer sender.wg.Done()

	log.Debug("SENDER: OPEN")
	sender.senderInit()
	for done := false; !done; {
		select {
		case msg, ok := <-sender.senderChan:
			if !ok {
				log.Debug("SENDER: NOMORE")
				done = true
				break
			}
			log.Debug("SENDER *******")
			log.WithFields(log.Fields{"record": msg}).Debug("SENDER: got msg")
			sender.networkSendRecord(msg)
			// handleLogWriter(sender, msg)
		}
	}
	log.Debug("SENDER: FIN")
}
