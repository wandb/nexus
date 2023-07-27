package main

import (
	"context"
	"flag"
	"os"
	"runtime/trace"

	"github.com/wandb/wandb/nexus/pkg/client"
	"golang.org/x/exp/slog"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to connect to")
	samples := flag.Int("smpl", 1000000, "number of samples to log")
	flag.Parse()

	ctx := context.Background()
	manager := client.NewManager(ctx, *addr)
	settings := client.NewSettings()
	run := manager.NewRun(ctx, settings)

	run.Setup()
	run.Init()
	run.Start()

	f, err := os.Create("trace.out")
	if err != nil {
		slog.Error("failed to create trace output file", "err", err)
		panic(err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			slog.Error("failed to close trace file", "err", err)
			panic(err)
		}
	}()

	if err = trace.Start(f); err != nil {
		slog.Error("failed to start trace", "err", err)
		panic(err)
	}
	defer trace.Stop()

	for i := 0; i < *samples; i++ {
		data := map[string]float64{
			"loss": float64(i),
		}
		run.Log(data)
	}
	run.Finish()

	manager.Close()
}
