package server

import (
	"github.com/wandb/wandb/nexus/pkg/service"
)

type Dispatcher struct {
	resultChan chan *service.Result
	responders map[string]Responder
}

func NewDispatcher() *Dispatcher {
	dispatcher := Dispatcher{resultChan: make(chan *service.Result)}
	return &dispatcher
}

func (disp *Dispatcher) AddResponder(responderId string, responder Responder) {
	disp.responders[responderId] = responder
}

func (disp *Dispatcher) Start() {
	go func() {
		for result := range disp.resultChan {
			// extract responder id from result
			//responderId := result.GetControl().ResponderId
			responderId := "localhvost"
			response := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
			}
			disp.responders[responderId].Respond(response)
		}
	}()
}
