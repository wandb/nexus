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
	ctx context.Context
	//cancel     context.CancelFunc
	wg         sync.WaitGroup
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *service.Settings
	logger     *slog.Logger
	finished   bool
}

func NewStream(ctx context.Context, settings *service.Settings, streamId string, responders ...ResponderEntry) *Stream {
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
	handler.dispatcherChan = dispatcher.inChan
	sender.dispatcherChan = dispatcher.inChan

	//ctx, cancel := context.WithCancel(ctx)
	stream := &Stream{
		ctx: ctx,
		//cancel:     cancel,
		wg:         sync.WaitGroup{},
		dispatcher: dispatcher,
		handler:    handler,
		sender:     sender,
		writer:     writer,
		settings:   settings,
		logger:     logger,
	}
	stream.AddResponders(responders...)
	stream.wg.Add(1)
	go stream.Start()
	return stream
}

func (s *Stream) AddResponders(entries ...ResponderEntry) {
	for _, entry := range entries {
		s.dispatcher.AddResponder(entry)
	}
}

//func (s *Stream) RemoveResponder(responderId string) {
//	s.dispatcher.RemoveResponder(responderId)
//}

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {
	defer s.wg.Done()

	// start the handler
	s.wg.Add(1)
	go func() {
		s.handler.start()
		s.wg.Done()
	}()

	// do the writer
	s.wg.Add(1)
	go func() {
		s.writer.start()
		s.wg.Done()
	}()

	// do the sender
	s.wg.Add(1)
	go func() {
		s.sender.start()
		s.wg.Done()
	}()

	// do the dispatcher
	s.wg.Add(1)
	go func() {
		s.dispatcher.start()
		s.wg.Done()
	}()
}

func (s *Stream) HandleRecord(rec *service.Record) {
	slog.Debug("Stream.HandleRecord", "record", rec.String())
	s.handler.inChan <- rec
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
func (s *Stream) Close(force bool) {
	// send exit record to handler
	if force {
		record := &service.Record{RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}}, Control: &service.Control{AlwaysSend: true}}
		s.HandleRecord(record)
	}
	s.wg.Wait()
	if force {
		s.PrintFooter()
	}
}

func (s *Stream) PrintFooter() {
	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
}
