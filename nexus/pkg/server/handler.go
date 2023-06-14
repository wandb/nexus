package server

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"strconv"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
	// "time"

	log "github.com/sirupsen/logrus"
)

type Handler struct {
	handlerChan    chan *service.Record
	dispatcherChan chan *service.Result

	currentStep int64
	startTime   float64

	wg  *sync.WaitGroup
	run *service.RunRecord

	summary map[string]string

	settings *Settings
}

func NewHandler(settings *Settings, dispatcherChan chan *service.Result) *Handler {
	wg := sync.WaitGroup{}

	handler := Handler{
		wg:             &wg,
		settings:       settings,
		summary:        make(map[string]string),
		handlerChan:    make(chan *service.Record),
		dispatcherChan: dispatcherChan}
	return &handler
}

func (h *Handler) Start() {
	go func() {
		log.Debug("handler started")
		for record := range h.handlerChan {
			log.WithFields(log.Fields{"rec": record}).Debug("HANDLER")
			//h.storeRecord(record)
			h.handleRecord(record)
		}
	}()
}

func (h *Handler) Stop() {
	close(h.handlerChan)
}

// storeRecord writes the record to the writer if it is set.
func (h *Handler) storeRecord(msg *service.Record) {
	switch msg.RecordType.(type) {
	case *service.Record_Request:
		// don't log this
	case nil:
		// The field is not set.
		log.Fatal("storeRecord: record type is nil")
	default:
		//if h.writer != nil {
		//	h.writer.WriteRecord(msg)
		//}
	}
}

func (h *Handler) handleRecord(msg *service.Record) {
	switch x := msg.RecordType.(type) {
	case *service.Record_Alert:
	case *service.Record_Artifact:
	case *service.Record_Config:
	case *service.Record_Exit:
		h.handleRunExit(msg, x.Exit)
	case *service.Record_Files:
		//h.sender.SendRecord(msg)
	case *service.Record_Final:
	case *service.Record_Footer:
	case *service.Record_Header:
	case *service.Record_History:
	case *service.Record_LinkArtifact:
	case *service.Record_Metric:
	case *service.Record_Output:
	case *service.Record_OutputRaw:
	case *service.Record_Preempting:
	case *service.Record_Request:
		log.WithFields(log.Fields{"req": x}).Debug("reqgot")
		h.handleRequest(msg, x.Request)
	case *service.Record_Run:
		h.handleRun(msg, x.Run)
	case *service.Record_Stats:
	case *service.Record_Summary:
	case *service.Record_Tbrecord:
	case *service.Record_Telemetry:
	case nil:
		log.Fatal("handleRecord: record type is nil")
	default:
		log.Fatalf("handleRecord: unknown record type %T", x)
	}
}

func (h *Handler) handleRequest(rec *service.Record, req *service.Request) {
	ref := req.ProtoReflect()
	desc := ref.Descriptor()
	num := ref.WhichOneof(desc.Oneofs().ByName("request_type")).Number()
	log.WithFields(log.Fields{"type": num}).Debug("PROCESS: REQUEST")

	response := &service.Response{}
	switch x := req.RequestType.(type) {
	case *service.Request_CheckVersion:
	case *service.Request_Defer:
		h.handleDefer(rec, x.Defer)
	case *service.Request_GetSummary:
		h.handleGetSummary(rec, x.GetSummary, response)
	case *service.Request_Keepalive:
	case *service.Request_NetworkStatus:
	case *service.Request_PartialHistory:
		log.WithFields(log.Fields{"req": x}).Debug("PROCESS: got partial")
		h.handlePartialHistory(rec, x.PartialHistory)
		return
	case *service.Request_PollExit:
	case *service.Request_RunStart:
		log.WithFields(log.Fields{"req": x}).Debug("PROCESS: got start")
		h.handleRunStart(rec, x.RunStart)
	case *service.Request_SampledHistory:
	case *service.Request_ServerInfo:
	case *service.Request_Shutdown:
	case *service.Request_StopStatus:
	default:
		log.Fatalf("handleRequest: unknown request type %T", x)
	}

	result := &service.Result{
		ResultType: &service.Result_Response{Response: response},
		Control:    rec.Control,
		Uuid:       rec.Uuid,
	}
	h.dispatcherChan <- result
}

func (h *Handler) handleRunStart(rec *service.Record, req *service.RunStartRequest) {
	var ok bool
	run := req.Run
	h.startTime = float64(run.StartTime.AsTime().UnixMicro()) / 1e6
	h.run, ok = proto.Clone(run).(*service.RunRecord)
	if !ok {
		log.Fatal("handleRunStart: failed to clone run")
	}
	//h.sender.SendRecord(rec)
}

func (h *Handler) handleRun(rec *service.Record, run *service.RunRecord) {
	// runResult := &service.RunUpdateResult{Run: run}

	// let sender take care of it
	//h.sender.SendRecord(rec)
}

func (h *Handler) handleRunExit(rec *service.Record, runExit *service.RunExitRecord) {
	control := rec.GetControl()
	if control != nil {
		control.AlwaysSend = true
	}
	//h.sender.SendRecord(rec)
}

func (h *Handler) handleGetSummary(_ *service.Record, _ *service.GetSummaryRequest, response *service.Response) {
	var items []*service.SummaryItem

	for key, element := range h.summary {
		items = append(items, &service.SummaryItem{Key: key, ValueJson: element})
	}

	response.ResponseType = &service.Response_GetSummaryResponse{GetSummaryResponse: &service.GetSummaryResponse{Item: items}}
}

func (h *Handler) handleDefer(rec *service.Record, req *service.DeferRequest) {
	switch req.State {
	case service.DeferRequest_END:
		//if h.writer != nil {
		//	h.writer.Flush()
		//}
	default:
	}
	//h.sender.SendRecord(rec)
}

func (h *Handler) handlePartialHistory(_ *service.Record, req *service.PartialHistoryRequest) {
	items := req.Item

	stepNum := h.currentStep
	h.currentStep += 1
	s := service.HistoryStep{Num: stepNum}

	var runTime float64 = 0
	// walk through items looking for _timestamp
	for i := 0; i < len(items); i++ {
		if items[i].Key == "_timestamp" {
			val, err := strconv.ParseFloat(items[i].ValueJson, 64)
			if err != nil {
				log.Errorf("Error parsing _timestamp: %s", err)
			}
			runTime = val - h.startTime
		}
	}
	items = append(items,
		&service.HistoryItem{Key: "_runtime", ValueJson: fmt.Sprintf("%f", runTime)},
		&service.HistoryItem{Key: "_step", ValueJson: fmt.Sprintf("%d", stepNum)},
	)

	record := service.HistoryRecord{Step: &s, Item: items}

	r := service.Record{
		RecordType: &service.Record_History{History: &record},
	}
	h.storeRecord(&r)
	h.updateSummary(&record)

	//if h.sender != nil {
	//	h.sender.SendRecord(&r)
	//}
}

func (h *Handler) updateSummary(msg *service.HistoryRecord) {
	items := msg.Item
	for i := 0; i < len(items); i++ {
		h.summary[items[i].Key] = items[i].ValueJson
	}
}

func (h *Handler) GetRun() *service.RunRecord {
	return h.run
}

func (h *Handler) HandleRecord(rec *service.Record) {
	h.handlerChan <- rec
}
