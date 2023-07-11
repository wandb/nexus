package server

import (
	"context"

	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
)

type Dispatcher struct {
	ctx        context.Context
	inChan     chan *service.Result
	responders map[string]Responder
	logger     *slog.Logger
}

func NewDispatcher(ctx context.Context, logger *slog.Logger) *Dispatcher {
	dispatcher := &Dispatcher{
		ctx:        ctx,
		inChan:     make(chan *service.Result),
		responders: make(map[string]Responder),
		logger:     logger,
	}
	return dispatcher
}

func (d *Dispatcher) AddResponder(entry ResponderEntry) {
	responderId := entry.ID
	if _, ok := d.responders[responderId]; !ok {
		d.responders[responderId] = entry.Responder
	} else {
		slog.LogAttrs(
			d.ctx,
			slog.LevelError,
			"Responder already exists",
			slog.String("responder", responderId))
	}
}

func (d *Dispatcher) Deliver(result *service.Result) {
	d.inChan <- result
}

func (d *Dispatcher) start() {
	// start the dispatcher
	for msg := range d.inChan {
		responderId := msg.Control.ConnectionId
		LogResult(d.logger, "dispatch: got msg", msg)
		response := &service.ServerResponse{
			ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: msg},
		}
		if responderId == "" {
			LogResult(slog.Default(), "dispatch: got msg with no connection id", msg)
			continue
		}
		d.responders[responderId].Respond(response)
	}
}
