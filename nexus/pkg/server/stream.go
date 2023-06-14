package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	handler      *Handler
	responder    *Responder
	settings     *Settings
	finished     bool
	responseChan chan *service.Result
	closeChan    chan bool
}

func NewStream(settings *Settings) *Stream {
	responseChan := make(chan *service.Result)
	closeChan := make(chan bool)
	responder := NewResponder(responseChan)
	handler := NewHandler(responseChan, closeChan, responder.RespondResult, settings)
	return &Stream{
		responder:    responder,
		handler:      handler,
		settings:     settings,
		responseChan: responseChan,
		closeChan:    closeChan,
	}
}

func (s *Stream) Start(respondServerResponse func(context.Context, *service.ServerResponse)) {
	s.responder.Start(respondServerResponse)
	s.handler.Start()
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
	s.HandleRecord(&record)
	// wait for the handler to finish
	<-s.closeChan
	// print the footer
	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
