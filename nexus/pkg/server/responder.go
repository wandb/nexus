package server

import "github.com/wandb/wandb/nexus/pkg/service"

type Responder interface {
	Respond(response *service.ServerResponse)
}

type recordChannel interface {
	Deliver(data *service.Record)
}

type dispatchChannel interface {
	Deliver(data *service.Result)
}
