package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

func Average(nums []float64) float64 {
	if len(nums) == 0 {
		return 0.0
	}
	total := 0.0
	for _, num := range nums {
		total += num
	}
	return total / float64(len(nums))
}

type Metric interface {
	Name() string
	Sample()
	Clear()
	Aggregate() float64
}

type Asset interface {
	Name() string
	Metrics() []Metric
	IsAvailable() bool
	Start()
	Stop()
	Probe() map[string]map[string]interface{}
}

type MetricsMonitor struct {
	ctx      context.Context
	cancel   context.CancelFunc
	metrics  []Metric
	settings *service.Settings
	logger   *observability.NexusLogger
	outChan  chan<- *service.Record
}

func NewMetricsMonitor(
	ctx context.Context,
	cancel context.CancelFunc,
	metrics []Metric,
	settings *service.Settings,
	logger *observability.NexusLogger,
	outChan chan<- *service.Record,
) *MetricsMonitor {
	return &MetricsMonitor{
		ctx:      ctx,
		cancel:   cancel,
		metrics:  metrics,
		settings: settings,
		logger:   logger,
		outChan:  outChan,
	}
}

func (mm *MetricsMonitor) makeStatsRecord(stats map[string]float64) *service.Record {
	record := &service.Record{
		RecordType: &service.Record_Stats{
			Stats: &service.StatsRecord{
				StatsType: service.StatsRecord_SYSTEM,
				Timestamp: &timestamppb.Timestamp{Seconds: time.Now().Unix()},
			},
		},
		Control: &service.Control{AlwaysSend: true},
	}

	for k, v := range stats {
		jsonData, err := json.Marshal(v)
		if err != nil {
			continue
		}
		record.GetStats().Item = append(record.GetStats().Item, &service.StatsItem{
			Key:       k,
			ValueJson: string(jsonData),
		})
	}

	return record
}

func (mm *MetricsMonitor) Monitor() {
	// reset ctx:
	mm.ctx, mm.cancel = context.WithCancel(mm.ctx)

	// todo: rename the setting...should be SamplingIntervalSeconds
	samplingInterval := time.Duration(mm.settings.XStatsSampleRateSeconds.GetValue() * float64(time.Second))
	samplesToAverage := mm.settings.XStatsSamplesToAverage.GetValue()
	fmt.Println(samplingInterval, samplesToAverage)

	// Create a ticker that fires every `samplingInterval` seconds
	ticker := time.NewTicker(samplingInterval)
	defer ticker.Stop()

	// Create a new channel and immediately send a signal
	tickChan := make(chan time.Time, 1)
	tickChan <- time.Now()

	// Forward signals from the ticker to tickChan
	go func() {
		for t := range ticker.C {
			tickChan <- t
		}
	}()

	samplesCollected := int32(0)

	for {
		select {
		case <-mm.ctx.Done():
			return
		case <-tickChan:
			fmt.Println("current time:", time.Now().Format(time.RFC3339Nano))
			for _, metric := range mm.metrics {
				metric.Sample()
			}
			samplesCollected++

			if samplesCollected == samplesToAverage {
				aggregatedMetrics := mm.aggregate()
				if len(aggregatedMetrics) > 0 {
					// publish metrics
					fmt.Println(aggregatedMetrics)
					record := mm.makeStatsRecord(aggregatedMetrics)
					fmt.Println(record)
					mm.outChan <- record

				}
				for _, metric := range mm.metrics {
					metric.Clear()
				}
				// reset samplesCollected
				samplesCollected = int32(0)
			}
		}
	}
}

func (mm *MetricsMonitor) aggregate() map[string]float64 {
	aggregatedMetrics := make(map[string]float64)

	for _, metric := range mm.metrics {
		aggregatedMetrics[metric.Name()] = metric.Aggregate()
	}
	return aggregatedMetrics
}

func (mm *MetricsMonitor) Stop() {
	mm.logger.Info("Stopping metrics monitor")
	mm.cancel()
}

type SystemMonitor struct {
	// ctx is the context for the system monitor
	ctx    context.Context
	cancel context.CancelFunc

	// assets is the list of assets to monitor
	assets []Asset

	//	outChan is the channel for outgoing messages
	OutChan chan<- *service.Record

	// logger is the logger for the system monitor
	logger *observability.NexusLogger

	// settings is the settings for the system monitor
	settings *service.Settings
}

// NewSystemMonitor creates a new SystemMonitor with the given settings
func NewSystemMonitor(
	ctx context.Context,
	outChan chan<- *service.Record,
	settings *service.Settings,
	logger *observability.NexusLogger,
) *SystemMonitor {
	ctx, cancel := context.WithCancel(ctx)

	assets := []Asset{
		NewMemory(ctx, cancel, settings, logger, outChan),
	}

	systemMonitor := &SystemMonitor{
		ctx:      ctx,
		cancel:   cancel,
		assets:   assets,
		logger:   logger,
		settings: settings,
	}

	return systemMonitor
}

func (sm *SystemMonitor) Do() {
	sm.logger.Info("Starting system monitor")

	// start monitoring the assets
	for _, asset := range sm.assets {
		go asset.Start()
	}

}

func (sm *SystemMonitor) Stop() {
	sm.logger.Info("Stopping system monitor")
	sm.cancel()
}
