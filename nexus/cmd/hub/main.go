package main

import (
	"github.com/wandb/wandb/nexus/pkg/hub"
)

func main() {
	service := hub.TcpService{}
	service.Serve()
}
