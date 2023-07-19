package main

import (
	"context"
	"flag"
	"github.com/getsentry/sentry-go"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/server"
	"golang.org/x/exp/slog"
	"os"
	"runtime/trace"
)

// this is set by the build script and used by the observability package
var commit string

func main() {
	portFilename := flag.String(
		"port-filename",
		"port_file.txt",
		"filename for port to communicate with client",
	)
	pid := flag.Int("pid", 0, "pid")
	debug := flag.Bool("debug", false, "debug mode")
	noAnalytics := flag.Bool("no-observability", false, "turn off observability")
	// todo: remove these flags, they are here for backwards compatibility
	serveSock := flag.Bool("serve-sock", false, "use sockets")
	serveGrpc := flag.Bool("serve-grpc", false, "use grpc")

	flag.Parse()

	if debug != nil && *debug {
		// performance profiling

		// - use with the go trace tool: `go tool trace trace.out`
		// - use with `gotraceui`:
		//   - `go install honnef.co/go/gotraceui/cmd/gotraceui@latest`
		//   - `go run honnef.co/go/gotraceui/cmd/gotraceui@latest`
		//   - open the trace.out file in the UI
		f, err := os.Create("trace.out")
		if err != nil {
			panic(err)
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				panic(err)
			}
		}(f)

		err = trace.Start(f)
		if err != nil {
			panic(err)
		}
		defer trace.Stop()
	}

	logger := server.SetupDefaultLogger()
	ctx := context.Background()

	// set up sentry reporting
	observability.InitSentry(*noAnalytics, commit)
	defer sentry.Flush(2)

	logger.LogAttrs(
		ctx,
		slog.LevelDebug,
		"Flags",
		slog.String("fname", *portFilename),
		slog.Int("pid", *pid),
		slog.Bool("debug", *debug),
		slog.Bool("noAnalytics", *noAnalytics),
		slog.Bool("serveSock", *serveSock),
		slog.Bool("serveGrpc", *serveGrpc),
	)

	nexus := server.NewServer(ctx, "127.0.0.1:0", *portFilename)
	nexus.Close()
}
