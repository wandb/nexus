package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/auth"
	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"net"
	"strings"
	"sync"
)

type Connection struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	conn   net.Conn
	id     string

	requestChan  chan *service.ServerRequest
	respondChan  chan *service.ServerResponse
	shutdownChan chan struct{}
}

func NewConnection(
	ctx context.Context,
	conn net.Conn,
	shutdownChan chan struct{},
) *Connection {
	ctx, cancel := context.WithCancel(ctx)

	nc := &Connection{
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
		conn:         conn,
		id:           conn.RemoteAddr().String(), // check if this is properly unique
		requestChan:  make(chan *service.ServerRequest),
		respondChan:  make(chan *service.ServerResponse),
		shutdownChan: shutdownChan, //TODO: eventually remove this, we should be able to handle shutdown outside of the connection
	}
	nc.handle()
	return nc
}

func (nc *Connection) handle() {

	nc.wg.Add(1)
	go func() {
		defer nc.wg.Done()
		nc.handleConnection()
	}()

	nc.wg.Add(1)
	go func() {
		defer nc.wg.Done()
		nc.handleServerRequest()
	}()

	nc.wg.Add(1)
	go func() {
		defer nc.wg.Done()
		nc.handleServerResponse()
	}()
}

func (nc *Connection) Close() {
	<-nc.ctx.Done()
	err := nc.conn.Close()
	if err != nil {
		LogError(slog.Default(), "problem closing connection", err)
	}
	close(nc.requestChan)
	close(nc.respondChan) // TODO: this is not safe, we should have a way to signal that we are done

	slog.Debug("connection: Close")
	nc.wg.Wait()
}

func (nc *Connection) handleConnection() {
	scanner := bufio.NewScanner(nc.conn)
	tokenizer := &Tokenizer{}
	scanner.Split(tokenizer.split)
	for scanner.Scan() {
		msg := &service.ServerRequest{}
		err := proto.Unmarshal(scanner.Bytes(), msg)
		if err != nil {
			slog.LogAttrs(context.Background(),
				slog.LevelError,
				"Unmarshalling error",
				slog.String("err", err.Error()))
		} else {
			nc.requestChan <- msg
		}
	}
}

func (nc *Connection) Respond(resp *service.ServerResponse) {
	nc.respondChan <- resp
}

func (nc *Connection) handleServerResponse() {
	slog.Debug("handleServerResponse: start")
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
	slog.Debug("handleServerResponse: finished")
}

func (nc *Connection) handleServerRequest() {
	slog.Debug("handleServerRequest: start")
	for msg := range nc.requestChan {
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
	slog.Debug("handleServerRequest: finished")
}

func (nc *Connection) handleInformInit(msg *service.ServerInformInitRequest) {
	slog.Debug("connection: handleInformInit: init")
	settings := msg.GetSettings()

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
	}(settings) // TODO: this is a hack, we should not be modifying the settings

	streamId := msg.GetXInfo().GetStreamId()
	slog.Debug("connection: handleInformInit: init: streamId", slog.String("streamId", streamId))

	stream := NewStream(nc.ctx, settings, streamId)
	if err := streamMux.addStream(streamId, stream); err != nil {
		slog.Error("handleInformInit: stream already exists")
		return
	}
	stream.AddResponder(nc.id, nc)
	go stream.Start()
}

func (nc *Connection) handleInformStart(_ *service.ServerInformStartRequest) {
	slog.Debug("handleInformStart: start")
}

func (nc *Connection) handleInformRecord(msg *service.Record) {
	streamId := msg.XInfo.StreamId
	if stream, err := streamMux.getStream(streamId); err != nil {
		slog.Error("handleInformRecord: stream not found")
	} else {
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
	}
}

func (nc *Connection) handleInformFinish(msg *service.ServerInformFinishRequest) {
	slog.Debug("handleInformFinish: finish")
	streamId := msg.XInfo.StreamId
	if stream, err := streamMux.getStream(streamId); err != nil {
		slog.Error("handleInformFinish:", err.Error())
	} else {
		stream.MarkFinished()
		stream.RemoveResponder(nc.id)
	}
}

func (nc *Connection) handleInformTeardown(_ *service.ServerInformTeardownRequest) {
	slog.Debug("handleInformTeardown: teardown")
	close(nc.shutdownChan)
	slog.Debug("handleInformTeardown: shutdownChan sent")
	streamMux.Close()
	slog.Debug("handleInformTeardown: streamMux closed")
	nc.cancel()
	slog.Debug("handleInformTeardown: context canceled")
}
