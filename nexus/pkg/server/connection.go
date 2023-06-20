package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"
)

type Connection struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn
	id     string

	shutdownChan chan<- bool
	requestChan  chan *service.ServerRequest
	respondChan  chan *service.ServerResponse
}

func NewConnection(
	ctx context.Context,
	cancel context.CancelFunc,
	conn net.Conn,
	shutdownChan chan<- bool,
) *Connection {
	return &Connection{
		ctx:          ctx,
		cancel:       cancel,
		conn:         conn,
		id:           conn.RemoteAddr().String(), // check if this is properly unique
		shutdownChan: shutdownChan,
		requestChan:  make(chan *service.ServerRequest),
		respondChan:  make(chan *service.ServerResponse),
	}
}

func (nc *Connection) receive(wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(nc.conn)
	tokenizer := Tokenizer{}
	scanner.Split(tokenizer.split)

	// Run Scanner in a separate goroutine to listen for incoming messages
	go func() {
		for scanner.Scan() {
			msg := &service.ServerRequest{}
			err := proto.Unmarshal(scanner.Bytes(), msg)
			if err != nil {
				log.Error("Unmarshalling error: ", err)
				continue
			}
			nc.requestChan <- msg
		}
		nc.cancel()
	}()

	// wait for context to be canceled
	<-nc.ctx.Done()

	log.Debug("receive: Context canceled")
}

func (nc *Connection) transmit(wg *sync.WaitGroup) {
	defer wg.Done()

	go func() {
		for msg := range nc.respondChan {
			out, err := proto.Marshal(msg)
			if err != nil {
				log.Error("Error marshalling msg:", err)
				return
			}

			writer := bufio.NewWriter(nc.conn)
			header := Header{Magic: byte('W'), DataLength: uint32(len(out))}
			if err = binary.Write(writer, binary.LittleEndian, &header); err != nil {
				log.Error("Error writing header:", err)
				return
			}
			if _, err = writer.Write(out); err != nil {
				log.Error("Error writing msg:", err)
				return
			}

			if err = writer.Flush(); err != nil {
				log.Error("Error flushing writer:", err)
				return
			}
		}
	}()

	// wait for context to be canceled
	<-nc.ctx.Done()
	log.Debug("transmit: Context canceled")
}

func (nc *Connection) process(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case msg := <-nc.requestChan:
			nc.handleMessage(msg)
		case <-nc.ctx.Done():
			log.Debug("PROCESS: Context canceled")
			return
		}
	}
}

func (nc *Connection) Respond(resp *service.ServerResponse) {
	nc.respondChan <- resp
}

func handleConnection(ctx context.Context, cancel context.CancelFunc, swg *sync.WaitGroup, conn net.Conn, shutdownChan chan<- bool) {
	connection := NewConnection(ctx, cancel, conn, shutdownChan)

	defer func() {
		swg.Done()
		err := connection.conn.Close()
		if err != nil {
			log.Error(err)
		}
		close(connection.requestChan)
		close(connection.respondChan)
	}()

	wg := sync.WaitGroup{}
	wg.Add(3)

	go connection.receive(&wg)
	go connection.process(&wg)
	go connection.transmit(&wg)

	wg.Wait()

	log.Debug("handleConnection: DONE")
}

func (nc *Connection) handleInformInit(msg *service.ServerInformInitRequest) {
	log.Debug("connection: handleInformInit: init")
	s := msg.XSettingsMap
	settings := NewSettings(s)

	streamId := msg.XInfo.StreamId
	stream := streamMux.addStream(streamId, settings)
	stream.AddResponder(nc.id, nc)
	go stream.Start()
}

func (nc *Connection) handleInformStart(_ *service.ServerInformStartRequest) {
	log.Debug("handleInformStart: start")
}

func (nc *Connection) handleInformRecord(msg *service.Record) {
	streamId := msg.XInfo.StreamId
	if stream, ok := streamMux.getStream(streamId); ok {
		ref := msg.ProtoReflect()
		desc := ref.Descriptor()
		num := ref.WhichOneof(desc.Oneofs().ByName("record_type")).Number()
		log.WithFields(log.Fields{"type": num}).Debug("PROCESS: COMM/PUBLISH")
		// add connection id to control message
		// so that the stream can send back a response
		// to the correct connection
		if msg.Control != nil {
			msg.Control.ConnectionId = nc.id
		} else {
			msg.Control = &service.Control{ConnectionId: nc.id}
		}
		log.Println("handleInformRecord: ", msg)
		stream.HandleRecord(msg)
	} else {
		log.Error("handleInformRecord: stream not found")
	}
}

func (nc *Connection) handleInformFinish(msg *service.ServerInformFinishRequest) {
	log.Debug("handleInformFinish: finish")
	streamId := msg.XInfo.StreamId
	if stream, ok := streamMux.getStream(streamId); ok {
		stream.MarkFinished()
	} else {
		log.Error("handleInformFinish: stream not found")
	}
}

func (nc *Connection) handleInformTeardown(_ *service.ServerInformTeardownRequest) {
	log.Debug("handleInformTeardown: teardown")
	streamMux.Close()
	log.Debug("handleInformTeardown: streamMux closed")
	nc.cancel()
	log.Debug("handleInformTeardown: context canceled")
	nc.shutdownChan <- true
	log.Debug("handleInformTeardown: shutdownChan signaled")
}

func (nc *Connection) handleMessage(msg *service.ServerRequest) {
	switch x := msg.ServerRequestType.(type) {
	case *service.ServerRequest_InformInit:
		nc.handleInformInit(x.InformInit)
	case *service.ServerRequest_InformStart:
		nc.handleInformStart(x.InformStart)
	case *service.ServerRequest_RecordPublish:
		nc.handleInformRecord(x.RecordPublish)
	case *service.ServerRequest_RecordCommunicate:
		nc.handleInformRecord(x.RecordCommunicate)
	case *service.ServerRequest_InformFinish:
		nc.handleInformFinish(x.InformFinish)
	case *service.ServerRequest_InformTeardown:
		nc.handleInformTeardown(x.InformTeardown)
	case nil:
		log.Fatal("ServerRequestType is nil")
	default:
		log.Fatalf("ServerRequestType is unknown, %T", x)
	}
}
