package monitor

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
	"time"
)

type SystemMonitor struct {
	// ctx is the context for the system monitor
	ctx    context.Context
	cancel context.CancelFunc

	//	outChan is the channel for outgoing messages
	OutChan chan<- *service.Record

	// logger is the logger for the system monitor
	logger *observability.NexusLogger

	// settings is the settings for the system monitor
	settings *service.Settings
}

// NewSystemMonitor creates a new SystemMonitor with the given settings
func NewSystemMonitor(ctx context.Context, settings *service.Settings, logger *observability.NexusLogger) *SystemMonitor {
	ctx, cancel := context.WithCancel(ctx)
	return &SystemMonitor{
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		settings: settings,
	}
}

func (sm *SystemMonitor) Do() {
	sm.logger.Info("Starting system monitor")

	// todo: rename the setting...should be SamplingIntervalSeconds
	samplingInterval := time.Duration(sm.settings.XStatsSampleRateSeconds.GetValue()) * time.Second
	// samplesToAverage := sm.settings.XStatsSamplesToAverage.GetValue()
	sm.logger.Info(fmt.Sprintf("Sampling interval: %v", samplingInterval))

	// Create a ticker that fires every `samplingInterval` seconds
	ticker := time.NewTicker(samplingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			sm.logger.Info("Stopping system monitor")
			return
		case <-ticker.C:
			v, _ := mem.VirtualMemory()

			// almost every return value is a struct
			fmt.Printf("Total: %v, Free:%v, UsedPercent:%f%%\n", v.Total, v.Free, v.UsedPercent)

			// convert to JSON. String() is also implemented
			fmt.Println(v)

			// CPU
			cpuInfo, _ := cpu.Info()
			fmt.Println(cpuInfo)
			// sm.logger.Info(fmt.Sprintf("CPU Info: %v", cpuInfo))

			// // CPU percentage
			// percent, _ := cpu.Percent(1, true)
			//
			// for i, cpuPercent := range percent {
			// 	// sm.logger.Info(fmt.Sprintf("CPU %d: %f", i, cpuPercent))
			// 	fmt.Printf("CPU %d: %f%%\n", i, cpuPercent)
			// }

			sm.logger.Info("Sending system stats")
		}
	}
}

func (sm *SystemMonitor) Close() {
	sm.logger.Info("Closing system monitor")
	sm.cancel()
}
