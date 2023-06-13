package server

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/leveldb"

	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"
)

type Writer struct {
	writerChan chan *service.Record
	wg         *sync.WaitGroup
	settings   *Settings
}

func NewWriter(settings *Settings) *Writer {
	wg := sync.WaitGroup{}
	writer := Writer{
		wg:         &wg,
		settings:   settings,
		writerChan: make(chan *service.Record),
	}

	wg.Add(1)
	go writer.writerGo()
	return &writer
}

func (w *Writer) Stop() {
	close(w.writerChan)
}

func (w *Writer) WriteRecord(rec *service.Record) {
	w.writerChan <- rec
}

func logHeader(f *os.File) {
	type logHeader struct {
		ident   [4]byte
		magic   uint16
		version byte
	}
	buf := new(bytes.Buffer)
	ident := [4]byte{byte(':'), byte('W'), byte('&'), byte('B')}
	head := logHeader{ident: ident, magic: 0xBEE1, version: 0}
	if err := binary.Write(buf, binary.LittleEndian, &head); err != nil {
		log.Error(err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		log.Error(err)
	}
}

func (w *Writer) writerGo() {
	f, err := os.Create(w.settings.SyncFile)
	if err != nil {
		log.Fatal(err)
	}
	defer w.wg.Done()
	defer f.Close()

	logHeader(f)

	records := leveldb.NewWriterExt(f, leveldb.CRCAlgoIEEE)

	log.Debug("WRITER: OPEN")
	for {
		msg, ok := <-w.writerChan
		if !ok {
			log.Debug("NOMORE")
			break
		}
		log.Debug("WRITE *******")
		// handleLogWriter(w, msg)

		rec, err := records.Next()
		if err != nil {
			log.Fatal(err)
		}

		out, err := proto.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}

		_, err = rec.Write(out)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Debug("WRITER: CLOSE")

	if err = records.Close(); err != nil {
		return
	}
	log.Debug("WRITER: FIN")
}

func (w *Writer) Flush() {
	log.Debug("WRITER: flush")
	close(w.writerChan)
	w.wg.Wait()
}
