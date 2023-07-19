package monitor

import (
	"context"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
	"sync"
)

// Metrics

type MemoryPercent struct {
	name    string
	samples []float64
	mutex   sync.RWMutex
}

func (mp *MemoryPercent) Name() string {
	return mp.name
}

func (mp *MemoryPercent) Sample() {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	// implementation of sample goes here
	virtualMem, _ := mem.VirtualMemory()
	mp.samples = append(mp.samples, virtualMem.UsedPercent)
}

func (mp *MemoryPercent) Clear() {
	// implementation of clear goes here
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	mp.samples = []float64{}
}

func (mp *MemoryPercent) Aggregate() float64 {
	// return sum(mp.samples) / float64(len(mp.samples))
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	return Average(mp.samples)
}

// Asset

type Memory struct {
	name           string
	metrics        []Metric
	metricsMonitor *MetricsMonitor
}

func NewMemory(
	ctx context.Context,
	cancel context.CancelFunc,
	settings *service.Settings,
	logger *observability.NexusLogger,
	outChan chan<- *service.Record,
) *Memory {
	metrics := []Metric{
		&MemoryPercent{
			name:    "memory_percent",
			samples: []float64{},
		},
	}
	metricsMonitor := NewMetricsMonitor(
		ctx,
		cancel,
		metrics,
		settings,
		logger,
		outChan,
	)

	return &Memory{
		name:           "memory",
		metrics:        metrics,
		metricsMonitor: metricsMonitor,
	}
}

func (m *Memory) Name() string {
	return m.name
}

func (m *Memory) Metrics() []Metric {
	return []Metric{
		&MemoryPercent{
			name:    "memory_percent",
			samples: []float64{},
		},
	}
}

func (m *Memory) IsAvailable() bool {
	return true
}

func (m *Memory) Start() {
	m.metricsMonitor.Monitor()
}

func (m *Memory) Stop() {
	m.metricsMonitor.Stop()
}

func (m *Memory) Probe() map[string]interface{} {
	info := make(map[string]interface{})
	info["total"] = 1337
	return info
}
