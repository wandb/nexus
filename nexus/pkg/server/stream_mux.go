package server

import (
	"fmt"
	"golang.org/x/exp/slog"
	"sync"
)

// StreamMux is a multiplexer for streams.
// It is thread-safe and is used to ensure that
// only one stream exists for a given streamId so that
// we can safely add responders to streams.
type StreamMux struct {
	mux   map[string]*Stream
	mutex sync.RWMutex
}

func NewStreamMux() *StreamMux {
	return &StreamMux{
		mux: make(map[string]*Stream),
	}
}

// addStream adds a stream to the mux if it doesn't already exist.
func (sm *StreamMux) addStream(streamId string, stream *Stream) error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if _, ok := sm.mux[streamId]; !ok {
		sm.mux[streamId] = stream
		return nil
	} else {
		return fmt.Errorf("stream already exists")
	}
}

func (sm *StreamMux) getStream(streamId string) (*Stream, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if stream, ok := sm.mux[streamId]; !ok {
		return nil, fmt.Errorf("stream not found")
	} else {
		return stream, nil
	}
}

// todo: add this when we have a way to remove mux
// func (sm *StreamMux) removeStream(streamId string) {
//  	sm.mutex.Lock()
//  	defer sm.mutex.Unlock()
//  	delete(sm.mux, streamId)
// }

// Close closes all streams in the mux.
func (sm *StreamMux) Close() {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	wg := sync.WaitGroup{}
	for _, stream := range sm.mux {
		wg.Add(1)
		go func(stream *Stream) {
			stream.Close()
			wg.Done()
		}(stream)
	}
	wg.Wait()
	slog.Debug("all streams were closed")
}

// StreamMux is a singleton.
var streamMux = NewStreamMux()
