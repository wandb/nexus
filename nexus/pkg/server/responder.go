package server

import (
	"context"
	// "fmt"

	"github.com/wandb/wandb/nexus/pkg/service"
	// "github.com/Khan/genqlient/graphql"
	//    log "github.com/sirupsen/logrus"
)

type Responder struct {
	responseChan chan *service.Result
}

func NewResponder(responseChan chan *service.Result) *Responder {
	responder := Responder{responseChan: responseChan}
	return &responder
}

func (resp *Responder) Start(respondServerResponse func(ctx context.Context, result *service.ServerResponse)) {
	go func() {
		for result := range resp.responseChan {
			//if ok := resp.mailbox.Respond(result); ok {
			//	continue
			//}
			resp := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
			}
			// fixme: this is a hack to get the context
			respondServerResponse(context.Background(), resp)
		}
	}()
}

func (resp *Responder) RespondResult(rec *service.Result) {
	resp.responseChan <- rec
}
