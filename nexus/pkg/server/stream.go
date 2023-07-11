package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
)

// Stream is a collection of components that work together to handle incoming
// data for a W&B run, store it locally, and send it to a W&B server.
// Stream.handler receives incoming data from the client and dispatches it to
// Stream.writer, which writes it to a local file. Stream.writer then sends the
// data to Stream.sender, which sends it to the W&B server. Stream.dispatcher
// handles dispatching responses to the appropriate client responders.
type Stream struct {
	ctx        context.Context
	cancel     context.CancelFunc
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *service.Settings
	logger     *slog.Logger
	finished   bool
}

func NewStream(ctx context.Context, settings *service.Settings, streamId string) *Stream {
	logFile := settings.GetLogInternal().GetValue()
	logger := SetupStreamLogger(logFile, streamId)

	dispatcher := NewDispatcher(ctx, logger)
	sender := NewSender(ctx, settings, logger)
	writer := NewWriter(ctx, settings, logger)
	handler := NewHandler(ctx, settings, logger)

	// connect the components
	handler.outChan = writer.inChan
	writer.outChan = sender.inChan
	sender.outChan = handler.inChan

	// connect the dispatcher to the handler and sender
	handler.dispatcherChan = dispatcher
	sender.dispatcherChan = dispatcher

	ctx, cancel := context.WithCancel(ctx)
	stream := &Stream{
		ctx:        ctx,
		cancel:     cancel,
		dispatcher: dispatcher,
		handler:    handler,
		sender:     sender,
		writer:     writer,
		settings:   settings,
		logger:     logger,
	}
	return stream
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

//func (s *Stream) RemoveResponder(responderId string) {
//	s.dispatcher.RemoveResponder(responderId)
//}

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {

	wg := &sync.WaitGroup{}

	// start the handler
	go s.handler.start()

	// do the writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.writer.start()
	}()

	// do the sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.sender.start()
	}()

	// do the dispatcher
	go s.dispatcher.start()

	wg.Wait()
	s.cancel()
}

func (s *Stream) HandleRecord(rec *service.Record) {
	s.handler.inChan <- rec
}

func (s *Stream) MarkFinished() {
	s.finished = true
}

func (s *Stream) IsFinished() bool {
	return s.finished
}

func (s *Stream) GetSettings() *service.Settings {
	return s.settings
}

func (s *Stream) GetRun() *service.RunRecord {
	return s.handler.GetRun()
}

// Close closes the stream's handler, writer, sender, and dispatcher.
// We first mark the Stream's context as done, which signals to the
// components that they should Close. Each of the components will
// call Done() on the Stream's wait group when they are finished closing.
func (s *Stream) Close() {
	// todo: is this the best way to handle this?
	if s.IsFinished() {
		return
	}

	// send exit record to handler
	record := &service.Record{RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}}, Control: &service.Control{AlwaysSend: true}}
	s.HandleRecord(record)
	<-s.ctx.Done()

	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
