package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Stream struct {
	ctx        context.Context
	cancel     context.CancelFunc
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *Settings
	wg         *sync.WaitGroup
	finished   bool
}

func NewStream(settings *Settings) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := NewDispatcher(ctx)
	sender := NewSender(ctx, settings, dispatcher)
	writer := NewWriter(ctx, settings, sender)
	handler := NewHandler(ctx, settings, writer, dispatcher)
	wg := &sync.WaitGroup{}

	return &Stream{
		ctx:        ctx,
		cancel:     cancel,
		dispatcher: dispatcher,
		handler:    handler,
		sender:     sender,
		writer:     writer,
		settings:   settings,
		wg:         wg,
	}
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {
	s.wg.Add(4)
	go s.handler.start(s.wg)
	go s.writer.start(s.wg)
	go s.sender.start(s.wg)
	go s.dispatcher.start(s.wg)
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

// Close closes the stream's handler, writer, sender, and dispatcher.
// We first mark the Stream's context as done, which signals to the
// components that they should close. Each of the components will
// call Done() on the Stream's wait group when they are finished closing.

func (s *Stream) Close(wg *sync.WaitGroup) {
	defer wg.Done()

	// todo: is this the best way to handle this?
	if s.IsFinished() {
		return
	}

	// send exit record to handler
	// record := service.Record{
	// 	RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
	// }
	// s.HandleRecord(&record)

	// signal to components that they should close
	s.cancel()
	// wait for components to finish closing
	s.wg.Wait()

	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
