package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	handler    *Handler
	dispatcher *Dispatcher
	settings   *Settings
	finished   bool
}

func NewStream(settings *Settings) *Stream {
	//responder := NewResponder(mailbox)
	handler := NewHandler(responder.RespondResult, settings)
	return &Stream{responder: responder, handler: handler, mailbox: mailbox, settings: settings}
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

func (s *Stream) Start(respondServerResponse func(context.Context, *service.ServerResponse)) {
	// s.responder.Start(respondServerResponse)
	s.handler.Start()
}

func (s *Stream) Deliver(rec *service.Record) *MailboxHandle {
	handle := s.mailbox.Deliver(rec)
	fmt.Println("deliver rec", rec)
	s.HandleRecord(rec)
	return handle
}

func (s *Stream) HandleRecord(rec *service.Record) {
	s.handler.HandleRecord(rec)
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
	record := service.Record{
		RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
	}
	_ = s.Deliver(&record).wait()
	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
