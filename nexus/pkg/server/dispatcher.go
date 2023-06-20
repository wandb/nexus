package server

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Dispatcher struct {
	inChan     chan *service.Result
	responders map[string]Responder
}

func NewDispatcher(ctx context.Context) *Dispatcher {
	dispatcher := &Dispatcher{
		inChan:     make(chan *service.Result),
		responders: make(map[string]Responder),
	}
	return dispatcher
}

func (d *Dispatcher) AddResponder(responderId string, responder Responder) {
	if _, ok := d.responders[responderId]; !ok {
		d.responders[responderId] = responder
	} else {
		log.Errorf("Responder %s already exists", responderId)
	}
}

func (d *Dispatcher) Deliver(result *service.Result) {
	d.inChan <- result
}
