package server

import (
	"context"
	// "fmt"

	"github.com/wandb/wandb/nexus/pkg/service"
	// "github.com/Khan/genqlient/graphql"
	//    log "github.com/sirupsen/logrus"
)

type Responder struct {
	responderChan chan *service.Result
	mailbox       *Mailbox
}

func NewResponder(mailbox *Mailbox) *Responder {
	responder := Responder{mailbox: mailbox, responderChan: make(chan *service.Result)}
	return &responder
}

func (resp *Responder) Start(respondServerResponse func(ctx context.Context, result *service.ServerResponse)) {
	go resp.responderGo(respondServerResponse)
}

func (resp *Responder) RespondResult(rec *service.Result) {
	resp.responderChan <- rec
}

func (resp *Responder) responderGo(respondServerResponse func(ctx context.Context, result *service.ServerResponse)) {
	for result := range resp.responderChan {
		if ok := resp.mailbox.Respond(result); ok {
			continue
		}
		resp := &service.ServerResponse{
			ServerResponseType: &service.ServerResponse_ResultCommunicate{ResultCommunicate: result},
		}
		// fixme: this is a hack to get the context
		respondServerResponse(context.Background(), resp)
	}
}
