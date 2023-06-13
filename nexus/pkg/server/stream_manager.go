package server

import (
	"sync"
)

type StreamManager struct {
	streams map[string]*Stream
	mutex   sync.RWMutex
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]*Stream),
	}
}

func (sm *StreamManager) addStream(streamId string, settings *Settings) *Stream {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, ok := sm.streams[streamId]
	if !ok {
		stream = NewStream(settings)
		sm.streams[streamId] = stream
	}
	return stream
}

func (sm *StreamManager) getStream(streamId string) (*Stream, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	stream, ok := sm.streams[streamId]
	return stream, ok
}

func (sm *StreamManager) Close() {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	wg := sync.WaitGroup{}
	for _, stream := range sm.streams {
		wg.Add(1)
		go stream.Close(&wg) // test this
	}

	wg.Wait()
}

var streamManager = NewStreamManager()
