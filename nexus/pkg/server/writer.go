package server

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Writer struct {
	ctx      context.Context
	settings *Settings
	inChan   chan *service.Record
	outChan  recordChannel
	store    *Store
}

func NewWriter(ctx context.Context, settings *Settings, outChan recordChannel) *Writer {
	writer := &Writer{
		ctx:      ctx,
		settings: settings,
		inChan:   make(chan *service.Record),
		outChan:  outChan,
		store:    NewStore(settings.SyncFile),
	}
	return writer
}

func (w *Writer) start(wg *sync.WaitGroup) {
	// wait group for inner goroutine looping over input channel
	loopWg := &sync.WaitGroup{}
	loopWg.Add(1)

	defer func() {
		// finalize & clear writer's resources
		w.close()
		// wait for inner goroutine to finish
		loopWg.Wait()
		// signal stream that we're done
		wg.Done()
	}()

	// start inner goroutine looping over input channel
	go func() {
		for msg := range w.inChan {
			w.writeRecord(msg)
		}
		// signal outer goroutine that we're done
		loopWg.Done()
	}()

	// wait for outer context to be cancelled
	<-w.ctx.Done()
}

func (w *Writer) Deliver(msg *service.Record) {
	w.inChan <- msg
}

func (w *Writer) close() {
	close(w.inChan)
	err := w.store.Close()
	if err != nil {
		return
	}
}

// writeRecord Writing messages to the append-only log,
// and passing them to the sender.
// We ensure that the messages are written to the log
// before they are sent to the server.
func (w *Writer) writeRecord(rec *service.Record) {
	switch rec.RecordType.(type) {
	case *service.Record_Request:
		w.sendRecord(rec)
	case nil:
		log.Error("nil record type")
	default:
		w.store.storeRecord(rec)
		w.sendRecord(rec)
	}
}

func (w *Writer) sendRecord(rec *service.Record) {
	control := rec.GetControl()
	if w.settings.Offline && control != nil && !control.AlwaysSend {
		return
	}
	w.outChan.Deliver(rec)
}

func (w *Writer) Flush() {
	log.Debug("WRITER: flush")
	close(w.inChan)
}
