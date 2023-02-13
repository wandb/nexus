package main

import (
    "C"
    "fmt"
	"github.com/wandb/wandb/nexus/server"
	"github.com/wandb/wandb/nexus/service"
)

type Handler struct {
	send chan int
	recv chan int
}

var m map[int]Handler = make(map[int]Handler)

func readit(c chan int, d chan int) {
	x := <- c
    fmt.Println(x)
	d <- x
}

func setup() (chan int, chan int) {

	c := make(chan int)
	d := make(chan int)
	if m == nil {
		m = make(map[int]Handler)
	}
	m[18] = Handler{c, d}

	go readit(c, d)
	return c, d
}

func start_send_recv(num int) {

	c := make(chan int)
	d := make(chan int)
	if m == nil {
		m = make(map[int]Handler)
	}
	m[num] = Handler{c, d}

	go readit(c, d)
}

func RespondServerResponse(serverResponse *service.ServerResponse) {
}

//export nexus_connect
func nexus_connect() int {
	settings := &server.Settings{
		BaseURL:  "https://api.wandb.ai",
		ApiKey:   "6bb89ffd621b666f54fd6a6a2db6bd2aebcad909",
		SyncFile: "something.wandb",
		Offline:  false}

	runRecord := service.RunRecord{RunId:"jnk132"}
	r := service.Record{
		RecordType: &service.Record_Run{&runRecord},
	}
	s := server.NewStream(RespondServerResponse, settings)
	s.ProcessRecord(&r)

	return 22
}

//export nexus_finish
func nexus_finish(n int) {
}

//export nexus_log
func nexus_log(n int) {
}

//export nexus_close
func nexus_close(n int) {
}

//export _nexus_list
func _nexus_list() []int {
	return []int{}
}

//export nexus_send
func nexus_send(n int, cstr *C.char, clen C.int) {
}

//export nexus_recv
func nexus_recv(n int, cstr *C.char, clen C.int) int {
	return 0
}

//export nexus_start
func nexus_start() int {
	n := nexus_connect()
	start_send_recv(n)
	return n
}

//export PrintInt
func PrintInt(x int) {
	c, _ := setup()
	c <- x
	// <- d
}

//export GetInt
func GetInt() int {
	s := m[18]
	x := <- s.recv
	return x
}

func main() {}
