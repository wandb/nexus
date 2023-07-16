package main

import (
	"context"
	"flag"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/server"
	"golang.org/x/exp/slog"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

// this is set by the build script and used by the observability package
var commit string

func main() {
	portFilename := flag.String(
		"port-filename",
		"port_file.txt",
		"filename for port to communicate with client",
	)
	flag.Int("pid", 0, "pid")
	flag.Bool("debug", false, "debug mode")
	flag.Bool("no-observability", false, "turn off observability")
	// todo: remove these flags, they are here for backwards compatibility
	flag.Bool("serve-sock", false, "use sockets")
	flag.Bool("serve-grpc", false, "use grpc")

	flag.Parse()

	//logger := server.SetupDefaultLogger()
	ctx := context.Background()
	slog.SetDefault(observability.NoOpLogger.Logger)

	// set up sentry reporting
	//observability.InitSentry(*noAnalytics, commit)
	//defer sentry.Flush(2)

	//slog.LogAttrs(
	//	ctx,
	//	slog.LevelDebug,
	//	"Flags",
	//	slog.String("fname", *portFilename),
	//	slog.Int("pid", *pid),
	//	slog.Bool("debug", *debug),
	//	slog.Bool("noAnalytics", *noAnalytics),
	//	slog.Bool("serveSock", *serveSock),
	//	slog.Bool("serveGrpc", *serveGrpc),
	//)

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	runtime.GOMAXPROCS(24)

	nexus := server.NewServer(ctx, "127.0.0.1:0", *portFilename)
	nexus.Close()
}
