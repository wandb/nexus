package server

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Dispatcher struct {
	task       *Task
	inChan     chan *service.Result
	responders map[string]Responder
}

func NewDispatcher(ctx context.Context) *Dispatcher {
	task := NewTask(ctx)
	dispatcher := &Dispatcher{
		task:       task,
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
	loopWg := &sync.WaitGroup{}
	loopWg.Add(1)

	defer func() {
		d.close()
		loopWg.Wait()
		d.task.wg.Done()
	}()

	go func() {
		for result := range d.inChan {
			// todo: extract responder id from result
			responderId := result.Control.ConnectionId
			log.Debug("dispatching result to responder ", responderId)
			log.Debug("+++result: ", result)
			response := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
			}
			d.responders[responderId].Respond(response)
		}
		loopWg.Done()
	}()

	<-d.task.ctx.Done()
}

func (d *Dispatcher) close() {
	close(d.inChan)
}
