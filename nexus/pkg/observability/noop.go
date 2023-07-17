package observability

import (
	"context"

	"golang.org/x/exp/slog"
)

var disabledLogger = slog.New(disabledHandler{})

var NoOpLogger = &NexusLogger{disabledLogger, nil}

type disabledHandler struct{}

func (disabledHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (disabledHandler) Handle(context.Context, slog.Record) error { return nil }

func (disabledHandler) WithAttrs([]slog.Attr) slog.Handler {
	return disabledHandler{}
}

func (disabledHandler) WithGroup(string) slog.Handler {
	return disabledHandler{}
}
