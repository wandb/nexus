package server

import (
	"context"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
)

type Watcher struct {
	ctx      context.Context
	watcher  *fsnotify.Watcher
	outChan  chan<- *service.Record
	settings *service.Settings
	logger   *slog.Logger
	wg       *sync.WaitGroup
}

func NewWatcher(ctx context.Context, settings *service.Settings, logger *slog.Logger) *Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	w := &Watcher{
		ctx:      ctx,
		watcher:  watcher,
		outChan:  make(chan *service.Record),
		settings: settings,
		logger:   logger,
		wg:       &sync.WaitGroup{},
	}

	w.Add(settings.GetRootDir().GetValue())

	w.wg.Add(1)
	go w.do()

	return w
}

func (w *Watcher) Add(path string) {
	// Add a path.
	err := w.watcher.Add(path)
	if err != nil {
		w.logger.Error("watcher: error adding path", err)
	}
}

func (w *Watcher) do() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("watcher: context done")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Debug("watcher: events channel closed")
				return
			}
			w.logger.Debug("watcher: event", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				w.logger.Debug("watcher: modified file:", event.Name)
				//w.outChan <- &...
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Debug("watcher: errors channel closed")
				return
			}
			w.logger.Error("watcher: error:", err)
		}
	}
}

func (w *Watcher) close() {
	w.wg.Wait()
	err := w.watcher.Close()
	if err != nil {
		w.logger.Error("watcher: error closing watcher", err)
	}
}
