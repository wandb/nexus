package server

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
)

type NexusStream struct {
	Send     chan *service.Record
	Recv     chan *service.Result
	Run      *service.RunRecord
	Settings *service.Settings
	Callback func(run *service.RunRecord, settings *service.Settings, result *service.Result)
}

func (ns *NexusStream) SendRecord(r *service.Record) {
	ns.Send <- r
}

func (ns *NexusStream) SetResultCallback(cb func(run *service.RunRecord, settings *service.Settings, result *service.Result)) {
	ns.Callback = cb
}

func (ns *NexusStream) Start(s *Stream) {
	// read from send channel and call Handle
	// in a goroutine
	go func() {
		for record := range ns.Send {
			s.HandleRecord(record)
		}
	}()
}

func (ns *NexusStream) CaptureResult(result *service.Result) {
	// fmt.Println("GOT CAPTURE", result)

	switch x := result.ResultType.(type) {
	case *service.Result_RunResult:
		if ns.Run == nil {
			ns.Run = x.RunResult.GetRun()
			// ns.printHeader()
			// fmt.Println("GOT RUN from RESULT", ns.run)
		}
	case *service.Result_ExitResult:
		// ns.printFooter()
	}

	if ns.Callback != nil {
		ns.Callback(ns.Run, ns.Settings, result)
	}
}

var chars = "abcdefghijklmnopqrstuvwxyz1234567890"

func ShortID(length int) string {

	charsLen := len(chars)
	b := make([]byte, length)
	_, err := rand.Read(b) // generates len(b) random bytes
	if err != nil {
		LogFatalError(slog.Default(), "rand error", err)
		err = fmt.Errorf("rand error: %s", err.Error())
		slog.LogAttrs(context.Background(),
			slog.LevelError,
			"ShortID: error",
			slog.String("error", err.Error()))
		panic(err)
	}

	for i := 0; i < length; i++ {
		b[i] = chars[int(b[i])%charsLen]
	}
	return string(b)
}
