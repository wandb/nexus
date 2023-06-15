package server

import (
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Writer struct {
	settings *Settings
	inChan   chan *service.Record
	//senderChan chan *service.Record
	outChan recordChannel
	store   *Store
}

func NewWriter(settings *Settings, outChan recordChannel) *Writer {
	writer := &Writer{
		settings: settings,
		inChan:   make(chan *service.Record),
		outChan:  outChan,
		store:    NewStore(settings.SyncFile),
	}
	return writer
}

func (w *Writer) start() {
	for msg := range w.inChan {
		w.writeRecord(msg)
	}
}

func (w *Writer) Deliver(msg *service.Record) {
	w.inChan <- msg
}

func (w *Writer) Stop() {
	close(w.inChan)
	err := w.store.Close()
	if err != nil {
		return
	}
}

func (w *Writer) writeRecord(rec *service.Record) {
	switch rec.RecordType.(type) {
	case *service.Record_Request:
		w.sendRecord(rec)
	case nil:
		log.Error("nil record type")
	default:
		w.sendRecord(rec)
		w.store.storeRecord(rec)
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
