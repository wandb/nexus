package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

type Task struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func NewTask(parentCtx context.Context) *Task {
	ctx, cancel := context.WithCancel(parentCtx)
	return &Task{
		ctx:    ctx,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
	}
}

type Stream struct {
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
	sender := NewSender(ctx, settings, dispatcher)
	writer := NewWriter(ctx, settings, sender)
	handler := NewHandler(ctx, settings, writer, dispatcher)

	return &Stream{
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

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {
	s.handler.task.wg.Add(1)
	s.writer.task.wg.Add(1)
	s.sender.task.wg.Add(1)
	s.dispatcher.task.wg.Add(1)

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

	// signal to components that they should close, then wait for them to finish
	// we proceed in the following order:
	// 1. handler
	// 2. writer
	// 3. sender
	// 4. dispatcher
	s.handler.task.cancel()
	s.handler.task.wg.Wait()

	s.writer.task.cancel()
	s.writer.task.wg.Wait()

	s.sender.task.cancel()
	s.sender.task.wg.Wait()

	s.dispatcher.task.cancel()
	s.dispatcher.task.wg.Wait()

	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
