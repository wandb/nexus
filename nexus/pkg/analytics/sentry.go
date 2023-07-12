package analytics

import (
	"github.com/getsentry/sentry-go"
	"golang.org/x/exp/slog"
)

type SentryClient struct {
	Dsn     string
	ErrChan chan error
	MsgChan chan string
}

func NewSentryClient() *SentryClient {
	s := &SentryClient{
		Dsn:     "https://0d0c6674e003452db392f158c42117fb@o151352.ingest.sentry.io/4505513612214272",
		ErrChan: make(chan error),
	}
	return s
}

func (s *SentryClient) Do() {
	defer func() {
		sentry.Flush(2)
	}()

	err := sentry.Init(sentry.ClientOptions{
		Dsn: s.Dsn,
	})

	if err != nil {
		slog.Error("sentry.Init failed", "err", err)
	}

	select {
	case err := <-s.ErrChan:
		slog.Debug("error received", "err", err)
		sentry.CaptureException(err)
	case msg := <-s.MsgChan:
		slog.Debug("msg received", "msg", msg)
		sentry.CaptureMessage(msg)
	}
}
