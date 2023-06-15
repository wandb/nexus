package server

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Dispatcher struct {
	ctx        context.Context
	inChan     chan *service.Result
	responders map[string]Responder
}

func NewDispatcher(ctx context.Context) *Dispatcher {
	dispatcher := &Dispatcher{
		ctx:        ctx,
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

func (d *Dispatcher) start() {
	defer d.close()

	go func() {
		for result := range d.inChan {
			// todo: extract responder id from result
			responderId := result.Control.ConnectionId
			log.Debug("dispatching result to responder ", responderId)
			response := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
			}
			d.responders[responderId].Respond(response)
		}
	}()

	<-d.ctx.Done()

}

func (d *Dispatcher) close() {
	close(d.inChan)
}
