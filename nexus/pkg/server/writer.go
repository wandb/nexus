package server

import (
	"bytes"
	"encoding/binary"
	"os"
	"sync"

	"github.com/golang/leveldb/record"
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"
)

type Writer struct {
	writerChan chan *service.Record
	wg         *sync.WaitGroup
	settings   *Settings
}

func NewWriter(wg *sync.WaitGroup, settings *Settings) *Writer {
	writer := Writer{
		wg:         wg,
		settings:   settings,
		writerChan: make(chan *service.Record),
	}

	wg.Add(1)
	go writer.writerGo()
	return &writer
}

func (writer *Writer) Stop() {
	close(writer.writerChan)
}

func (writer *Writer) WriteRecord(rec *service.Record) {
	writer.writerChan <- rec
}

func logHeader(f *os.File) {
	type logHeader struct {
		ident   [4]byte
		magic   uint16
		version byte
	}
	buf := new(bytes.Buffer)
	ident := [4]byte{byte(':'), byte('W'), byte('&'), byte('B')}
	head := logHeader{ident: ident, magic: 0xBEE1, version: 1}
	err := binary.Write(buf, binary.LittleEndian, &head)
	checkError(err)
	_, err = f.Write(buf.Bytes())
	checkError(err)
}

func (w *Writer) writerGo() {
	f, err := os.Create(w.settings.SyncFile)
	checkError(err)
	defer w.wg.Done()
	defer f.Close()

	logHeader(f)

	records := record.NewWriter(f)

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
		checkError(err)

		out, err := proto.Marshal(msg)
		checkError(err)

		_, err = rec.Write(out)
		checkError(err)
	}
	log.Debug("WRITER: CLOSE")
	records.Close()
	log.Debug("WRITER: FIN")
}
