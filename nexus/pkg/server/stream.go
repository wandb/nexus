package server

import (
	"context"
	log "github.com/sirupsen/logrus"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
)

// Stream is a collection of components that work together to handle incoming
// data for a W&B run, store it locally, and send it to a W&B server.
// Stream.handler receives incoming data from the client and dispatches it to
// Stream.writer, which writes it to a local file. Stream.writer then sends the
// data to Stream.sender, which sends it to the W&B server. Stream.dispatcher
// handles dispatching responses to the appropriate client responders.
type Stream struct {
	handler    *Handler
	dispatcher *Dispatcher
	writer     *Writer
	sender     *Sender
	settings   *Settings
	finished   bool
	wg         *sync.WaitGroup
	done       chan struct{}
}

func NewStream(settings *Settings) *Stream {
	ctx := context.Background()
	dispatcher := NewDispatcher(ctx)
	sender := NewSender(ctx, settings)
	writer := NewWriter(ctx, settings)
	handler := NewHandler(ctx, settings)

	handler.outChan = writer.inChan
	handler.dispatcherChan = dispatcher

	writer.outChan = sender.inChan

	sender.outChan = handler.inChan
	sender.dispatcherChan = dispatcher

	return &Stream{
		wg:         &sync.WaitGroup{},
		dispatcher: dispatcher,
		handler:    handler,
		sender:     sender,
		writer:     writer,
		settings:   settings,
		done:       make(chan struct{}),
	}
}

func (s *Stream) AddResponder(responderId string, responder Responder) {
	s.dispatcher.AddResponder(responderId, responder)
}

// Start starts the stream's handler, writer, sender, and dispatcher.
// We use Stream's wait group to ensure that all of these components are cleanly
// finalized and closed when the stream is closed in Stream.Close().
func (s *Stream) Start() {

	s.wg.Add(3)

	// start the handler
	go func(wg *sync.WaitGroup) {
		defer s.handler.close()

		for msg := range s.handler.inChan {
			log.WithFields(log.Fields{"rec": msg}).Debug("handle: got msg")
			s.handler.handleRecord(msg)
		}
		log.Debug("++++++++++++++handler: started and closed")
		wg.Done()
	}(s.wg)

	// start the writer
	go func(wg *sync.WaitGroup) {
		defer s.writer.close()

		for msg := range s.writer.inChan {
			log.WithFields(log.Fields{"record": msg}).Debug("write: got msg")
			s.writer.writeRecord(msg)
		}
		log.Debug("++++++++++++++writer: started and closed")
		wg.Done()
	}(s.wg)

	// start the sender
	go func(wg *sync.WaitGroup) {
		//defer s.sender.close()

		for msg := range s.sender.inChan {
			log.WithFields(log.Fields{"record": msg}).Debug("sender: got msg")
			s.sender.sendRecord(msg)
		}
		log.Debug("++++++++++++++sender: started and closed")
		wg.Done()
	}(s.wg)

	// start the dispatcher
	go func() {
		for msg := range s.dispatcher.inChan {
			responderId := msg.Control.ConnectionId
			log.WithFields(log.Fields{"record": msg}).Debug("dispatch: got msg")
			response := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: msg},
			}
			if responderId == "" {
				log.WithFields(log.Fields{"record": msg}).Debug("dispatch: got msg with no connection id")
				continue
			}
			s.dispatcher.responders[responderId].Respond(response)
		}
	}()

	log.Debug("++++++++++++++stream: started and waiting")
	s.wg.Wait()
	log.Debug("++++++++++++++stream: started and closed")
	s.done <- struct{}{}
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

	//send exit record to handler
	record := service.Record{
		RecordType: &service.Record_Exit{Exit: &service.RunExitRecord{}},
		Control:    &service.Control{AlwaysSend: true},
	}
	s.HandleRecord(&record)
	close(s.handler.inChan)
	<-s.done

	settings := s.GetSettings()
	run := s.GetRun()
	PrintHeadFoot(run, settings)
	s.MarkFinished()
}
