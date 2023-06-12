package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"github.com/wandb/wandb/nexus/pkg/server"
)

func main() {
	/*
		Main entry point for the nexus service.
	*/
	portFilename := flag.String("port-filename", "portfile.txt", "filename")

	pid := flag.Int("pid", 0, "pid")
	debug := flag.Bool("debug", false, "debug")
	serveSock := flag.Bool("serve-sock", false, "debug")
	serveGrpc := flag.Bool("serve-grpc", false, "debug")

	flag.Parse()

	server.InitLogging()

	log.WithFields(log.Fields{
		"fname":     *portFilename,
		"pid":       *pid,
		"debug":     *debug,
		"serveSock": *serveSock,
		"serveGrpc": *serveGrpc,
	}).Debug("Flags")

	server.WandbService(*portFilename)

}
