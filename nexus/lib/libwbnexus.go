package main

import (
	"C"
	"crypto/rand"
	"os"
	"fmt"
	"github.com/wandb/wandb/nexus/server"
	"github.com/wandb/wandb/nexus/service"
)

type NexusStream struct {
	send chan service.Record
	recv chan service.Result
}

var m map[int]NexusStream = make(map[int]NexusStream)

func RespondServerResponse(serverResponse *service.ServerResponse) {
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
		ns.recv <-*result
	}
}

//export _nexus_list
func _nexus_list() []int {
	return []int{}
}

//
//
//

func (ns *NexusStream) sendRecord(r *service.Record) {
	ns.send <- *r
}

func (ns *NexusStream) start(s *server.Stream) {
	// read from send channel and call ProcessRecord
	// in a goroutine
	go func() {
		for {
			select {
			case record := <-ns.send:
				s.ProcessRecord(&record)
			}
		}
	}()
}


//export nexus_recv
func nexus_recv(num int) int {
	ns := m[num]
	got := <-ns.recv
	fmt.Println("RECV", &got)
	return 1
}

var chars = "abcdefghijklmnopqrstuvwxyz1234567890"

func shortID(length int) string {
    ll := len(chars)
    b := make([]byte, length)
    rand.Read(b) // generates len(b) random bytes
    for i := 0; i < length; i++ {
        b[i] = chars[int(b[i])%ll]
    }
    return string(b)
}

//export nexus_start
func nexus_start() int {
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
		run_id = shortID(8)
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
		m = make(map[int]NexusStream)
	}
	ns := NexusStream{c, d}
	m[num] = ns
	ns.start(s)

	ns.sendRecord(&r)
	// s.ProcessRecord(&r)

	// go processStuff()
	return num
}

//export nexus_finish
func nexus_finish(n int) {
	ns := m[n]
	exitRecord := service.RunExitRecord{}
	r := service.Record{
		RecordType: &service.Record_Exit{&exitRecord},
	}
	ns.sendRecord(&r)
}

//export nexus_log
func nexus_log(n int) {
	ns := m[n]
	historyRecord := service.HistoryRecord{}
	r := service.Record{
		RecordType: &service.Record_History{&historyRecord},
	}
	ns.sendRecord(&r)
}

//export nexus_log_scaler
func nexus_log_scaler(n int, log_key *C.char, log_value C.float) {
	ns := m[n]
	historyRecord := service.HistoryRecord{}
	r := service.Record{
		RecordType: &service.Record_History{&historyRecord},
	}
	ns.sendRecord(&r)
}

func main() {}
