package server

import (
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/analytics"
	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
	"io"
	"os"
)

func setupLogger(opts *slog.HandlerOptions, writers ...io.Writer) *slog.Logger {

	writer := io.MultiWriter(writers...)
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	}
	logger := slog.New(slog.NewJSONHandler(writer, opts))
	return logger
}

func SetupDefaultLogger() *slog.Logger {
	var writers []io.Writer

	name := "/tmp/logs.txt"
	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("FATAL Problem", err)
	} else {
		writers = append(writers, file)
	}
	if os.Getenv("WANDB_NEXUS_DEBUG") != "" {
		writers = append(writers, os.Stderr)
	}

	logger := setupLogger(nil, writers...)
	slog.SetDefault(logger)
	slog.Info("started logging")
	return logger
}

func SetupStreamLogger(name string, settings *service.Settings) *analytics.NexusLogger {
	var writers []io.Writer

	file, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("FATAL Problem", err)
	} else {
		writers = append(writers, file)
	}
	if os.Getenv("WANDB_NEXUS_DEBUG") != "" {
		writers = append(writers, os.Stderr)
	}

	writer := io.MultiWriter(writers...)

	tags := make(analytics.Tags)
	tags["run_id"] = settings.GetRunId().GetValue()
	tags["run_url"] = settings.GetRunUrl().GetValue()
	tags["project"] = settings.GetProject().GetValue()
	tags["entity"] = settings.GetEntity().GetValue()

	return analytics.NewNexusLogger(setupLogger(nil, writer), tags)
}
