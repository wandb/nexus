package server

import (
	"context"
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
)

type Writer struct {
	ctx      context.Context
	settings *service.Settings
	inChan   chan *service.Record
	outChan  chan<- *service.Record
	store    *Store
	logger   *slog.Logger
}

// NewWriter Creating a new writer.
func NewWriter(ctx context.Context, settings *service.Settings, logger *slog.Logger) *Writer {

	writer := &Writer{
		ctx:      ctx,
		settings: settings,
		inChan:   make(chan *service.Record),
		store:    NewStore(settings.GetSyncFile().GetValue(), logger),
		logger:   logger,
	}
	return writer
}

// Deliver Delivering messages to the writer.
func (w *Writer) Deliver(msg *service.Record) {
	w.inChan <- msg
}

// do Starting the writer.
func (w *Writer) do() {
	for msg := range w.inChan {
		LogRecord(w.logger, "write: got msg", msg)
		w.writeRecord(msg)
	}
	w.close()
	w.logger.Debug("writer: started and closed")
}

// close Closing the writer.
func (w *Writer) close() {
	close(w.outChan)
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
		w.logger.Debug(fmt.Sprintf("WRITER: request: %v\n", rec))
		w.sendRecord(rec)
	case nil:
		w.logger.Error("nil record type")
	default:
		w.store.storeRecord(rec)
		w.sendRecord(rec)
	}
}

// sendRecord Sending messages to the sender.
func (w *Writer) sendRecord(rec *service.Record) {
	control := rec.GetControl()
	LogRecord(w.logger, "WRITER: sendRecord", rec)
	if w.settings.GetXOffline().GetValue() && control != nil && !control.AlwaysSend {
		return
	}
	w.logger.Debug("WRITER: sendRecord: send")
	w.outChan <- rec
}
