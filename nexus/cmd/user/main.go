package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"

	"github.com/wandb/wandb/nexus/pkg/client"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to connect to")
	samples := flag.Int("smpl", 1000000, "number of samples to log")
	teardown := flag.Bool("td", false, "flag to close the server")
	flag.Parse()

	ctx := context.Background()
	manager := client.NewManager(ctx, *addr)
	settings := client.NewSettings()
	run := manager.NewRun(ctx, settings.Settings)

	run.Setup()
	run.Init()
	run.Start()

	for i := 0; i < *samples; i++ {
		data := map[string]float64{
			// generate random float and assign to key "foo":
			"loss": rand.Float64(),
		}
		if i%1000 == 0 {
			fmt.Println(i)
		}
		run.Log(data)
		// sleep for 0.1 seconds
		// time.Sleep(100 * time.Millisecond)
	}
	run.Finish()

	if *teardown {
		manager.Close()
	}
}
