package server

import (
	"context"
	// "fmt"

	"github.com/wandb/wandb/nexus/pkg/service"
	// "github.com/Khan/genqlient/graphql"
	//    log "github.com/sirupsen/logrus"
)

type Responder struct {
	resultChan chan *service.Result
	mailbox    *Mailbox
}

func NewResponder(mailbox *Mailbox) *Responder {
	responder := Responder{mailbox: mailbox, resultChan: make(chan *service.Result)}
	return &responder
}

func (resp *Responder) Start(respondServerResponse func(ctx context.Context, result *service.ServerResponse)) {
	go func() {
		for result := range resp.resultChan {
			if ok := resp.mailbox.Respond(result); ok {
				continue
			}
			resp := &service.ServerResponse{
				ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
			}
			// fixme: this is a hack to get the context
			respondServerResponse(context.Background(), resp)
		}
	}()
}

func (resp *Responder) RespondResult(rec *service.Result) {
	resp.resultChan <- rec
}
