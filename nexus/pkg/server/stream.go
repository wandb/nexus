package server

import (
	"context"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
)

// Stream is a collection of components that work together to handle incoming
// data for a W&B run, store it locally, and send it to a W&B server.
// Stream.handler receives incoming data from the client and dispatches it to
// Stream.writer, which writes it to a local file. Stream.writer then sends the
// data to Stream.sender, which sends it to the W&B server. Stream.dispatcher
// handles dispatching responses to the appropriate client responders.
type Stream struct {
	// ctx is the context for the stream
	ctx context.Context

	// wg is the WaitGroup for the stream
	wg sync.WaitGroup

	// handler is the handler for the stream
	handler *Handler

	// responders is the map of responders for the stream
	responders map[string]Responder

	// writer is the writer for the stream
	writer *Writer

	// sender is the sender for the stream
	sender *Sender

	// settings is the settings for the stream
	settings *service.Settings

	// logger is the logger for the stream
	logger *observability.NexusLogger

	// inChan is the channel for incoming messages
	inChan chan *service.Record
}

// NewStream creates a new stream with the given settings and responders.
func NewStream(ctx context.Context, settings *service.Settings, streamId string, responders ...ResponderEntry) *Stream {
	logFile := settings.GetLogInternal().GetValue()
	logger := SetupStreamLogger(logFile, settings)

	stream := &Stream{
		ctx:      ctx,
		wg:       sync.WaitGroup{},
		settings: settings,
		logger:   logger,
		inChan:   make(chan *service.Record),
	}
	stream.wg.Add(1)
	go stream.Start()
	return stream
}

// AddResponders adds the given responders to the stream's dispatcher.
func (s *Stream) AddResponders(entries ...ResponderEntry) {
	if s.responders == nil {
		s.responders = make(map[string]Responder)
	}
	for _, entry := range entries {
		responderId := entry.ID
		if _, ok := s.responders[responderId]; !ok {
			s.responders[responderId] = entry.Responder
		} else {
			s.logger.CaptureWarn("Responder already exists", "responder", responderId)
		}
	}
}

// handleDispatch handles dispatching messages from the handler and sender to
// the dispatcher.
func (s *Stream) handleDispatch() {
	for s.sender.dispatcherChan != nil || s.handler.dispatcherChan != nil {
		select {
		case value, ok := <-s.handler.dispatcherChan:
			if !ok {
				s.handler.dispatcherChan = nil
				continue
			}
			s.handleRespond(value)
		case value, ok := <-s.sender.dispatcherChan:
			if !ok {
				s.sender.dispatcherChan = nil
				continue
			}
			s.handleRespond(value)
		}
	}
}

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {
	defer s.wg.Done()
	s.logger.Info("created new stream", "id", s.settings.RunId)

	// handle the client requests
	s.handler = NewHandler(s.ctx, s.settings, s.logger)
	handleChan := s.handler.do(s.inChan)

	// write the data to a transaction log
	s.writer = NewWriter(s.ctx, s.settings, s.logger)
	writerChan := s.writer.do(handleChan)

	// send the data to the server
	s.sender = NewSender(s.ctx, s.settings, s.logger)
	s.sender.do(writerChan, s.inChan)

	// handle dispatching between components
	s.wg.Add(1)
	go func() {
		s.handleDispatch()
		s.wg.Done()
	}()
}

// HandleRecord handles the given record by sending it to the stream's handler.
func (s *Stream) HandleRecord(rec *service.Record) {
	s.logger.Debug("handling record", "record", rec)
	s.inChan <- rec
}

func (s *Stream) handleRespond(msg *service.Result) {
	responderId := msg.GetControl().GetConnectionId()
	s.logger.Debug("dispatch: got msg", "msg", msg)
	response := &service.ServerResponse{
		ServerResponseType: &service.ServerResponse_ResultCommunicate{
			ResultCommunicate: msg,
		},
	}
	if responderId == "" {
		s.logger.Debug("dispatch: got msg with no connection id", "msg", msg)
		return
	}
	s.responders[responderId].Respond(response)
}

func (s *Stream) GetRun() *service.RunRecord {
	return s.handler.GetRun()
}

// Close closes the stream's handler, writer, sender, and dispatcher.
// This can be triggered by the client (force=False) or by the server (force=True).
// We need the ExitRecord to initiate the shutdown procedure (which we
// either get from the client, or generate ourselves if the server is shutting us down).
// This will trigger the defer state machines (SM) in the stream's components:
//   - when the sender's SM gets to the final state, it will Close the handler
//   - this will trigger the handler to Close the writer
//   - this will trigger the writer to Close the sender
//
// This will finish the Stream's wait group, which will allow the stream to be
// garbage collected.
func (s *Stream) Close(force bool) {
	// send exit record to handler
	if force {
		record := &service.Record{
			RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
			Control:    &service.Control{AlwaysSend: true},
		}
		s.HandleRecord(record)
	}
	s.wg.Wait()
	if force {
		s.PrintFooter()
	}
	s.logger.Info("closed stream", "id", s.settings.RunId)
}

func (s *Stream) PrintFooter() {
	run := s.GetRun()
	PrintHeadFoot(run, s.settings)
}
