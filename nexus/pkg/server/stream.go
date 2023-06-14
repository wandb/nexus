package server

import (
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *Settings
	finished   bool
}

func NewStream(settings *Settings) *Stream {
	dispatcher := NewDispatcher()
	handler := NewHandler(settings, dispatcher.resultChan)
	sender := NewSender(handler.wg, settings, dispatcher.resultChan)
	var writer *Writer
	if settings.NoWrite {
		writer = NewWriter(settings)
	}
	return &Stream{dispatcher: dispatcher, handler: handler, sender: sender, settings: settings, writer: writer}
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

func (s *Stream) Start() {
	s.dispatcher.Start()
	s.handler.Start()
	s.sender.Start()
	if s.writer != nil {
		s.writer.Start()
	}
}

//func (s *Stream) Deliver(rec *service.Record) {
//	//handle := s.mailbox.Deliver(rec)
//	//fmt.Println("deliver rec", rec)
//	//s.HandleRecord(rec)
//	//return handle
//}

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
	//record := service.Record{
	//	RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
	//}
	//_ = s.Deliver(&record).wait()
	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
