package server

import (
	"sync"
)

type StreamMux struct {
	mux   map[string]*Stream
	mutex sync.RWMutex
}

func NewStreamMux() *StreamMux {
	return &StreamMux{
		mux: make(map[string]*Stream),
	}
}

func (sm *StreamMux) addStream(streamId string, settings *Settings) *Stream {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, ok := sm.mux[streamId]
	if !ok {
		stream = NewStream(settings)
		sm.mux[streamId] = stream
	}
	return stream
}

func (sm *StreamMux) getStream(streamId string) (*Stream, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, ok := sm.mux[streamId]
	return stream, ok
}

// todo: add this when we have a way to remove mux
// func (sm *StreamMux) removeStream(streamId string) {
//  	sm.mutex.Lock()
//  	defer sm.mutex.Unlock()
//  	delete(sm.mux, streamId)
// }

func (sm *StreamMux) Close() {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	wg := sync.WaitGroup{}
	for _, stream := range sm.mux {
		wg.Add(1)
		go stream.Close(&wg) // test this
	}

	wg.Wait()
}

var streamMux = NewStreamMux()
