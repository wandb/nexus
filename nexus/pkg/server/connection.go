package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/auth"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"net"
	"strings"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
)

type Connection struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn
	id     string

	requestChan  chan *service.ServerRequest
	respondChan  chan *service.ServerResponse
	shutdownChan chan<- bool
}

func NewConnection(
	ctx context.Context,
	conn net.Conn,
	shutdownChan chan<- bool,
) *Connection {
	ctx, cancel := context.WithCancel(ctx)

	return &Connection{
		ctx:          ctx,
		cancel:       cancel,
		conn:         conn,
		id:           conn.RemoteAddr().String(), // check if this is properly unique
		requestChan:  make(chan *service.ServerRequest),
		respondChan:  make(chan *service.ServerResponse),
		shutdownChan: shutdownChan,
	}
}

func (nc *Connection) receive() {
	scanner := bufio.NewScanner(nc.conn)
	tokenizer := Tokenizer{}
	scanner.Split(tokenizer.split)

	// Run Scanner in a separate goroutine to listen for incoming messages
	go func() {
		for scanner.Scan() {

			msg := &service.ServerRequest{}
			err := proto.Unmarshal(scanner.Bytes(), msg)
			if err != nil {
				slog.LogAttrs(context.Background(),
					slog.LevelError,
					"Unmarshalling error",
					slog.String("err", err.Error()))
				continue
			}
			nc.requestChan <- msg
		}
	}()

	<-nc.ctx.Done()
	slog.Debug("receive: finished")
	close(nc.requestChan)
	slog.Debug("receive: closed requestChan")
}

func (nc *Connection) transmit() {
	go func() {
		for msg := range nc.respondChan {
			out, err := proto.Marshal(msg)
			if err != nil {
				LogError(slog.Default(), "Error marshalling msg", err)
				return
			}

			writer := bufio.NewWriter(nc.conn)
			header := Header{Magic: byte('W'), DataLength: uint32(len(out))}
			if err = binary.Write(writer, binary.LittleEndian, &header); err != nil {
				LogError(slog.Default(), "Error writing header", err)
				return
			}
			if _, err = writer.Write(out); err != nil {
				LogError(slog.Default(), "Error writing msg", err)
				return
			}

			if err = writer.Flush(); err != nil {
				LogError(slog.Default(), "Error flushing writer", err)
				return
			}
		}
	}()

	<-nc.ctx.Done()
	slog.Debug("transmit: finished")
}

func (nc *Connection) process() {
	for msg := range nc.requestChan {
		nc.handleMessage(msg)
	}
	slog.Debug("process: finished")
}

func (nc *Connection) Respond(resp *service.ServerResponse) {
	nc.respondChan <- resp
}

func (nc *Connection) close() {
	err := nc.conn.Close()
	if err != nil {
		LogError(slog.Default(), "problem closing connection", err)
	}
	slog.Debug("connection: close")
	close(nc.respondChan)
}

func (nc *Connection) start() {
	defer nc.close()

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		nc.receive()
	}()
	go func() {
		defer wg.Done()
		nc.process()
	}()

	go func() {
		defer wg.Done()
		nc.transmit()
	}()
	wg.Wait()
	slog.Debug("connection: start: finished")
}

func (nc *Connection) handleInformInit(msg *service.ServerInformInitRequest) {
	slog.Debug("connection: handleInformInit: init")
	settings := msg.Settings

	func(s *service.Settings) {
		if s.GetApiKey().GetValue() != "" {
			return
		}
		host := strings.TrimPrefix(s.GetBaseUrl().GetValue(), "https://")
		host = strings.TrimPrefix(host, "http://")

		_, password, err := auth.GetNetrcLogin(host)
		if err != nil {
			LogFatal(slog.Default(), err.Error())
		}
		s.ApiKey = &wrapperspb.StringValue{Value: password}
	}(settings)

	slog.Debug("STREAM init")

	streamId := msg.XInfo.StreamId
	stream := streamMux.addStream(streamId, settings)
	stream.AddResponder(nc.id, nc)
	go stream.Start()
}

func (nc *Connection) handleInformStart(_ *service.ServerInformStartRequest) {
	slog.Debug("handleInformStart: start")
}

func (nc *Connection) handleInformRecord(msg *service.Record) {
	streamId := msg.XInfo.StreamId
	if stream, ok := streamMux.getStream(streamId); ok {
		ref := msg.ProtoReflect()
		desc := ref.Descriptor()
		num := ref.WhichOneof(desc.Oneofs().ByName("record_type")).Number()
		slog.LogAttrs(context.Background(),
			slog.LevelDebug,
			"PROCESS: COMM/PUBLISH",
			slog.Int("type", int(num)))
		// add connection id to control message
		// so that the stream can send back a response
		// to the correct connection
		if msg.Control != nil {
			msg.Control.ConnectionId = nc.id
		} else {
			msg.Control = &service.Control{ConnectionId: nc.id}
		}
		LogRecord(slog.Default(), "handleInformRecord", msg)
		stream.HandleRecord(msg)
	} else {
		slog.Error("handleInformRecord: stream not found")
	}
}

func (nc *Connection) handleInformFinish(msg *service.ServerInformFinishRequest) {
	slog.Debug("handleInformFinish: finish")
	streamId := msg.XInfo.StreamId
	if stream, ok := streamMux.getStream(streamId); ok {
		stream.MarkFinished()
	} else {
		slog.Error("handleInformFinish: stream not found")
	}
}

func (nc *Connection) handleInformTeardown(_ *service.ServerInformTeardownRequest) {
	slog.Debug("handleInformTeardown: teardown")
	nc.shutdownChan <- true
	slog.Debug("handleInformTeardown: shutdownChan sent")
	streamMux.Close()
	slog.Debug("handleInformTeardown: streamMux closed")
	nc.cancel()
	slog.Debug("handleInformTeardown: context canceled")
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
		panic("ServerRequestType is nil")
	default:
		panic(fmt.Sprintf("ServerRequestType is unknown, %T", x))
	}
}
