package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	ctx		   context.Context
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *Settings
	finished   bool
}

func NewStream(settings *Settings) *Stream {
	ctx := context.Background()
	dispatcher := NewDispatcher(ctx)
	sender := NewSender(settings, dispatcher)
	writer := NewWriter(settings, sender)
	handler := NewHandler(settings, writer, dispatcher)

	return &Stream{
		ctx:        ctx,
		dispatcher: dispatcher,
		handler:    handler,
		sender:     sender,
		writer:     writer,
		settings:   settings,
	}
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

func (s *Stream) Start() {
	go s.handler.start()
	go s.writer.start()
	go s.sender.start()
	go s.dispatcher.start()
}

func (s *Stream) HandleRecord(rec *service.Record) {
	s.handler.Handle(rec)
}

func (s *Stream) MarkFinished() {
	s.finished = true
}

func (s *Stream) IsFinished() bool {
	return s.finished
}

func (s *Stream) GetSettings() *Settings {
	return s.settings
}

func (s *Stream) GetRun() *service.RunRecord {
	return s.handler.GetRun()
}

func (s *Stream) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	if s.IsFinished() {
		return
	}
	// record := service.Record{
	// 	RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
	// }
	// _ = s.Deliver(&record).wait()
	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
