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
	ctx  context.Context
	conn net.Conn
	wg   sync.WaitGroup
	id   string

	inChan       chan *service.ServerRequest
	outChan      chan *service.ServerResponse
	teardownChan chan struct{}
}

func NewConnection(
	ctx context.Context,
	conn net.Conn,
	teardown chan struct{},
) *Connection {

	nc := &Connection{
		ctx:          ctx,
		wg:           sync.WaitGroup{},
		conn:         conn,
		id:           conn.RemoteAddr().String(), // TODO: check if this is properly unique
		inChan:       make(chan *service.ServerRequest),
		outChan:      make(chan *service.ServerResponse),
		teardownChan: teardown, //TODO: should we trigger teardown from a connection?
	}
	nc.wg.Add(1)
	go nc.handle()
	slog.Info("created new connection", "id", nc.id)
	return nc
}

func (nc *Connection) handle() {
	defer nc.wg.Done()

	nc.wg.Add(1)
	go func() {
		nc.handleServerRequest()
		nc.wg.Done()
	}()

	nc.wg.Add(1)
	go func() {
		nc.handleServerResponse()
		nc.wg.Done()
	}()
}

func (nc *Connection) Close() {
	slog.Debug("closing connection", "id", nc.id)
	if err := nc.conn.Close(); err != nil {
		slog.Error("error closing connection", "err", err.Error(), "id", nc.id)
	}
	nc.wg.Wait()
	slog.Info("closed connection", "id", nc.id)
}

func (nc *Connection) Respond(resp *service.ServerResponse) {
	nc.outChan <- resp
}

func (nc *Connection) handleServerResponse() {
	slog.Debug("starting handleServerResponse", "id", nc.id)
	for msg := range nc.outChan {
		out, err := proto.Marshal(msg)
		if err != nil {
			LogError(slog.Default(), "error marshalling msg", err)
			return
		}

		writer := bufio.NewWriter(nc.conn)
		header := Header{Magic: byte('W'), DataLength: uint32(len(out))}
		if err = binary.Write(writer, binary.LittleEndian, &header); err != nil {
			LogError(slog.Default(), "error writing header", err)
			return
		}
		if _, err = writer.Write(out); err != nil {
			LogError(slog.Default(), "error writing msg", err)
			return
		}

		if err = writer.Flush(); err != nil {
			LogError(slog.Default(), "error flushing writer", err)
			return
		}
	}
	slog.Debug("finished handleServerResponse", "id", nc.id)
}

func (nc *Connection) handleServerRequest() {
	defer close(nc.outChan)
	slog.Debug("starting handleServerRequest", "id", nc.id)
	for msg := range nc.inChan {
		slog.Debug("handling server request", "msg", msg.String(), "id", nc.id)
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
	slog.Debug("finished handleServerRequest", "id", nc.id)
}

func (nc *Connection) handleInformInit(msg *service.ServerInformInitRequest) {
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
	slog.Info("connection init received", slog.String("streamId", streamId), "id", nc.id)
	stream := NewStream(nc.ctx, settings, streamId, ResponderEntry{nc, nc.id})
	if err := streamMux.AddStream(streamId, stream); err != nil {
		slog.Error("connection init failed, stream already exists", slog.String("streamId", streamId))
		// TODO: should we close the stream?
		return
	}
}

func (nc *Connection) handleInformStart(_ *service.ServerInformStartRequest) {
}

func (nc *Connection) handleInformRecord(msg *service.Record) {
	streamId := msg.GetXInfo().GetStreamId()
	slog.Debug("handle record received", slog.String("streamId", streamId), "id", nc.id)
	if stream, err := streamMux.GetStream(streamId); err != nil {
		slog.Error("handleInformRecord: stream not found", slog.String("streamId", streamId))
	} else {
		// add connection id to control message
		// so that the stream can send back a response
		// to the correct connection
		if msg.Control != nil {
			msg.Control.ConnectionId = nc.id
		} else {
			msg.Control = &service.Control{ConnectionId: nc.id}
		}
		stream.HandleRecord(msg)
	}
}

func (nc *Connection) handleInformFinish(msg *service.ServerInformFinishRequest) {
	streamId := msg.XInfo.StreamId
	slog.Info("handle finish received", slog.String("streamId", streamId), "id", nc.id)
	if stream, err := streamMux.RemoveStream(streamId); err != nil {
		slog.Error("handleInformFinish:", "err", err.Error(), "streamId", streamId)
	} else {
		stream.Close(false)
	}
}

func (nc *Connection) handleInformTeardown(_ *service.ServerInformTeardownRequest) {
	slog.Debug("handle teardown received", "id", nc.id)
	close(nc.teardownChan)
	streamMux.CloseAllStreams(true) // TODO: this seems wrong to close all streams from a single connection
	nc.Close()
}
