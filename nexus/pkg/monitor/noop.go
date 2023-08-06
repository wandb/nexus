//go:build !linux && !amd64

package monitor

import (
	"github.com/wandb/wandb/nexus/pkg/service"
	"sync"
)

type GPUNvidia struct {
	name     string
	metrics  map[string][]float64
	settings *service.Settings
	mutex    sync.RWMutex
}

func NewGPUNvidia(settings *service.Settings) *GPUNvidia {
	gpu := &GPUNvidia{
		name:     "gpu",
		metrics:  map[string][]float64{},
		settings: settings,
	}

	return gpu
}

func (c *GPUNvidia) Name() string { return c.name }

func (c *GPUNvidia) SampleMetrics() {}

func (c *GPUNvidia) AggregateMetrics() map[string]float64 {
	return map[string]float64{}
}

func (c *GPUNvidia) ClearMetrics() {}

func (c *GPUNvidia) IsAvailable() bool { return false }

func (c *GPUNvidia) Probe() map[string]map[string]interface{} {
	info := make(map[string]map[string]interface{})
	return info
}
