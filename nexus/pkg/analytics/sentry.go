package analytics

import (
	"github.com/getsentry/sentry-go"
	"golang.org/x/exp/slog"
	"time"
)

type SentryClient struct {
	Dsn string
}

func InitSentry(disabled bool) {
	s := &SentryClient{}

	// The DSN to use. If the DSN is not set, the client is effectively disabled.
	if !disabled {
		s.Dsn = "https://0d0c6674e003452db392f158c42117fb@o151352.ingest.sentry.io/4505513612214272"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              s.Dsn,
		AttachStacktrace: true,
	})

	if err != nil {
		slog.Error("sentry.Init failed", "err", err)
	}

	if !disabled {
		slog.Debug("sentry.Init succeeded", "dsn", s.Dsn)
	} else {
		slog.Debug("sentry is disabled")
	}

}

func CaptureException(err error, tags map[string]string) {
	localHub := sentry.CurrentHub().Clone()
	localHub.ConfigureScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
	})
	localHub.CaptureException(err)
}

func Reraise() {
	err := recover()

	if err != nil {
		sentry.CurrentHub().Clone().Recover(err)
		sentry.Flush(time.Second * 2)

		panic(err)
	}
}
