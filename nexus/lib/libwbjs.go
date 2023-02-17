package main

import (
	"encoding/base64"
	"syscall/js"
	"github.com/wandb/wandb/nexus/server"
	"github.com/wandb/wandb/nexus/service"
)

var m map[int]*server.NexusStream = make(map[int]*server.NexusStream)

func main() {
	js.Global().Set("base64", encodeWrapper())
	js.Global().Set("wandb_init", wandb_init())
	<-make(chan bool)
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

func wandb_init() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		return wrap("junk", "")
	})
}

func encodeWrapper() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) == 0 {
			return wrap("", "Not enough arguments")
		}
		input := args[0].String()
		return wrap(base64.StdEncoding.EncodeToString([]byte(input)), "")
	})
}

func wrap(encoded string, err string) map[string]interface{} {
	return map[string]interface{}{
		"error":   err,
		"encoded": encoded,
	}
}

func doit() {
	base_url := "https://api.wandb.ai"
	run_id := server.ShortID(8)
	api_key := "invalid"
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
	m[num] = ns
	ns.Start(s)

	ns.SendRecord(&r)
	// s.ProcessRecord(&r)
}
