package server

import (
	"context"

	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
)

// Writer is responsible for writing messages to the append-only log.
// It receives messages from the handler, processes them,
// if the message is to be persisted it writes them to the log.
// It also sends the messages to the sender.
type Writer struct {
	// ctx is the context for the writer
	ctx context.Context

	// settings is the settings for the writer
	settings *service.Settings

	//// logger is the logger for the writer
	//logger *observability.NexusLogger

	// recordChan is the channel for outgoing messages
	recordChan chan *service.Record

	// store is the store for the writer
	store *Store
}

// NewWriter returns a new Writer
func NewWriter(ctx context.Context, settings *service.Settings, logger *observability.NexusLogger) *Writer {

	w := &Writer{
		ctx:      ctx,
		settings: settings,
		//logger:   logger,
	}
	return w
}

// do is the main loop of the writer to process incoming messages
func (w *Writer) do(inChan <-chan *service.Record) <-chan *service.Record {
	//w.logger.Info("writer: started", "stream_id", w.settings.RunId)
	w.recordChan = make(chan *service.Record, BufferSize)

	var err error
	w.store, err = NewStore(w.ctx, w.settings.GetSyncFile().GetValue(), nil)
	if err != nil {
		panic(err)
		//w.logger.CaptureFatalAndPanic("writer: error creating store", err)
	}
	go func() {
		for record := range inChan {
			w.handleRecord(record)
		}
		close(w.recordChan)
		w.close()
	}()
	return w.recordChan
}

// close closes the writer and all its resources
// which includes the store
func (w *Writer) close() {
	if err := w.store.Close(); err != nil {
		//w.logger.CaptureError("writer: error closing store", err)
		return
	}
	//w.logger.Info("writer: closed", "stream_id", w.settings.RunId)
}

// handleRecord Writing messages to the append-only log,
// and passing them to the sender.
// We ensure that the messages are written to the log
// before they are sent to the server.
func (w *Writer) handleRecord(record *service.Record) {
	//w.logger.Debug("write: got a message", "record", record, "stream_id", w.settings.RunId)
	switch record.RecordType.(type) {
	case *service.Record_Request:
		w.sendRecord(record)
	case nil:
		//w.logger.Error("nil record type")
	default:
		if err := w.storeRecord(record); err != nil {
			//w.logger.CaptureError("writer: error storing record", err)
		}
		w.sendRecord(record)
	}
}

// storeRecord stores the record in the append-only log
func (w *Writer) storeRecord(record *service.Record) error {
	if err := w.store.storeRecord(record); err != nil {
		return err
	}
	return nil
}

func (w *Writer) sendRecord(record *service.Record) {
	control := record.GetControl()
	// TODO: redo it so it only uses control
	if w.settings.GetXOffline().GetValue() && control != nil && !control.AlwaysSend {
		return
	}
	w.recordChan <- record
}
