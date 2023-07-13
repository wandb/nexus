package analytics

import (
	"context"

	"golang.org/x/exp/slog"
)

// A SentryLogger is a logger that sends errors to Sentry.
type SentryLogger struct {
	*slog.Logger
}

func NewSentryLogger(l *slog.Logger) *SentryLogger {
	return &SentryLogger{Logger: l}
}

func (l *SentryLogger) With(args ...any) *SentryLogger {
	return &SentryLogger{Logger: l.Logger.With(args...)}
}

func (l *SentryLogger) WithGroup(name string) *SentryLogger {
	return &SentryLogger{Logger: l.Logger.WithGroup(name)}
}

func (l *SentryLogger) CaptureErrorContext(ctx context.Context, err error, args ...any) {
	CaptureException(err, tagsFromArgs(args...))
	l.Logger.ErrorCtx(ctx, err.Error(), args...)
}

func (l *SentryLogger) CaptureError(err error, args ...any) {
	CaptureException(err, tagsFromArgs(args...))
	l.Logger.Error(err.Error(), args...)
}

func (l *SentryLogger) CaptureLogAttrs(ctx context.Context, level slog.Level, msg string, err error, attrs ...slog.Attr) {
	if level >= slog.LevelError {
		CaptureException(err, tagsFromAttrs(attrs...))
	}
	l.Logger.LogAttrs(ctx, level, msg, attrs...)
}

func (l *SentryLogger) CaptureLog(ctx context.Context, level slog.Level, msg string, err error, args ...any) {
	if level >= slog.LevelError {
		CaptureException(err, tagsFromArgs(args...))
	}
	l.Logger.Log(ctx, level, msg, args...)
}

func tagsFromArgs(args ...any) map[string]string {
	tags := make(map[string]string)
	for len(args) > 0 {
		switch x := args[0].(type) {
		case slog.Attr:
			tags[x.Key] = x.Value.String()
			args = args[1:]
		case string:
			if len(args) < 2 {
				break
			}
			attr := slog.Any(x, args[1])
			tags[attr.Key] = attr.Value.String()
			args = args[2:]
		default:
			args = args[1:]
		}
	}
	return tags
}

func tagsFromAttrs(attrs ...slog.Attr) map[string]string {
	tags := make(map[string]string)
	for _, attr := range attrs {
		tags[attr.Key] = attr.Value.String()
	}
	return tags
}
