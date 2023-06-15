package server

import (
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Dispatcher struct {
	inChan     chan *service.Result
	responders map[string]Responder
}

func NewDispatcher() *Dispatcher {
	dispatcher := &Dispatcher{inChan: make(chan *service.Result)}
	return dispatcher
}

func (d *Dispatcher) AddResponder(responderId string, responder Responder) {
	responder, ok := d.responders[responderId]
	if !ok {
		d.responders[responderId] = responder
	} else {
		log.Errorf("Responder %s already exists", responderId)
	}
}

func (d *Dispatcher) Deliver(result *service.Result) {
	d.inChan <- result
}

func (d *Dispatcher) start() {
	for result := range d.inChan {
		// extract responder id from result
		responderId := result.Uuid
		response := &service.ServerResponse{
			ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
		}
		d.responders[responderId].Respond(response)
	}
}
