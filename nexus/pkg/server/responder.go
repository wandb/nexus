package server

import "github.com/wandb/wandb/nexus/pkg/service"

type Responder interface {
	Respond(response *service.ServerResponse)
}
