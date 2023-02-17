package main

import (
	"C"
	"os"
	"time"
	"strings"
	"fmt"
	// "strings"
	"github.com/wandb/wandb/nexus/server"
	"github.com/wandb/wandb/nexus/service"
)

var m map[int]*server.NexusStream = make(map[int]*server.NexusStream)

func RespondServerResponse(serverResponse *service.ServerResponse) {
}

func PrintHeadFoot(run *service.RunRecord, settings *server.Settings) {
	// fmt.Println("GOT", ns.run)
	colorReset := "\033[0m"
	colorBlue := "\033[34m"
	colorYellow := "\033[33m"

	appURL := strings.Replace(settings.BaseURL, "//api.", "//", 1)
	url := fmt.Sprintf("%v/%v/%v/runs/%v", appURL, run.Entity, run.Project, run.RunId)
	fmt.Printf("%vwandb%v: 🚀 View run %v%v%v at: %v%v%v\n", colorBlue, colorReset, colorYellow, run.DisplayName, colorReset, colorBlue, url, colorReset)
}

func ResultCallback(run *service.RunRecord, settings *server.Settings, result *service.Result) {
	switch result.ResultType.(type) {
	case *service.Result_RunResult:
		// TODO: distinguish between first and subsequent RunResult
		PrintHeadFoot(run, settings)
	case *service.Result_ExitResult:
		PrintHeadFoot(run, settings)
		// FIXME: somehow need to quiesce all queues, this is a hack
		time.Sleep(4 * time.Second)
	}
}

func ResultFromServerResponse(serverResponse *service.ServerResponse) *service.Result {
	switch x := serverResponse.ServerResponseType.(type) {
	case *service.ServerResponse_ResultCommunicate:
		r := x.ResultCommunicate
		return r
	}
	return nil
}

func FuncRespondServerResponse(num int) func(serverResponse *service.ServerResponse) {
	return func(serverResponse *service.ServerResponse) {
		// fmt.Println("GOT", num, serverResponse)
		ns := m[num]

		result := ResultFromServerResponse(serverResponse)
		ns.CaptureResult(result)
		ns.Recv <-*result
	}
}

//export _nexus_list
func _nexus_list() []int {
	return []int{}
}

//
//
//


// func (ns *NexusStream) printHeader() {
// 	ns.printHeadFoot()
// }

// func (ns *NexusStream) printFooter() {
// 	ns.printHeadFoot()
// 
// 	// FIXME: somehow need to quiesce all queues, this is a hack
// 	time.Sleep(4 * time.Second)
// }

//export nexus_recv
func nexus_recv(num int) int {
	ns := m[num]
	_ = <-ns.Recv
	// fmt.Println("RECV", &got)
	return 1
}

//export nexus_start
func nexus_start() int {
	server.InitLogging()

	base_url := os.Getenv("WANDB_BASE_URL")
	if base_url == "" {
		base_url = "https://api.wandb.ai"
	}
	api_key := os.Getenv("WANDB_API_KEY")
	if api_key == "" {
		panic("set api key WANDB_API_KEY")
	}
	run_id := os.Getenv("WANDB_RUN_ID")
	if run_id == "" {
		run_id = server.ShortID(8)
	}

	settings := &server.Settings{
		BaseURL:  base_url,
		ApiKey:   api_key,
		SyncFile: "something.wandb",
		Offline:  false}

	runRecord := service.RunRecord{RunId:run_id}
	r := service.Record{
		RecordType: &service.Record_Run{&runRecord},
	}

	num := 42;
	s := server.NewStream(FuncRespondServerResponse(num), settings)

	c := make(chan service.Record)
	d := make(chan service.Result)
	if m == nil {
		m = make(map[int]*server.NexusStream)
	}
	ns := &server.NexusStream{c, d, nil, settings, nil}
	ns.SetResultCallback(ResultCallback)
	m[num] = ns
	ns.Start(s)

	ns.SendRecord(&r)
	// s.ProcessRecord(&r)

	// go processStuff()
	return num
}

//export nexus_run_start
func nexus_run_start(n int) {
	ns := m[n]
	run := m[n].Run
	// fmt.Println("SEND RUN START", n, run)

	if run == nil {
		panic("run cant be nil")
	}

	runStartRequest := service.RunStartRequest{}
	runStartRequest.Run = run
	req := service.Request{
		RequestType: &service.Request_RunStart{&runStartRequest},
	}
	r := service.Record{
		RecordType: &service.Record_Request{&req},
	}
	ns.SendRecord(&r)
}

//export nexus_finish
func nexus_finish(n int) {
	ns := m[n]
	exitRecord := service.RunExitRecord{}
	r := service.Record{
		RecordType: &service.Record_Exit{&exitRecord},
	}
	ns.SendRecord(&r)
}

//export nexus_log
func nexus_log(n int) {
	ns := m[n]
	historyRecord := service.HistoryRecord{}
	r := service.Record{
		RecordType: &service.Record_History{&historyRecord},
	}
	ns.SendRecord(&r)
}

//export nexus_log_scaler
func nexus_log_scaler(n int, log_key *C.char, log_value C.float) {
	ns := m[n]
	key := C.GoString(log_key)
	// fmt.Println("GOT", key, log_value)
	value_json := fmt.Sprintf("%v", log_value)
	historyRequest := service.PartialHistoryRequest{
		Item: []*service.HistoryItem{
			{
				Key: key,
				ValueJson: value_json,
			},
		},
	}
	req := service.Request{
		RequestType: &service.Request_PartialHistory{&historyRequest},
	}
	r := service.Record{
		RecordType: &service.Record_Request{&req},
	}
	ns.SendRecord(&r)
}

func main() {}
