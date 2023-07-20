package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	wg       sync.WaitGroup
	metrics  []Metric
	settings *service.Settings
	logger   *observability.NexusLogger
	outChan  chan<- *service.Record
}

func NewMetricsMonitor(
	metrics []Metric,
	settings *service.Settings,
	logger *observability.NexusLogger,
	outChan chan<- *service.Record,
) *MetricsMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &MetricsMonitor{
		ctx:      ctx,
		cancel:   cancel,
		wg:       sync.WaitGroup{},
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
	// todo: rename the setting...should be SamplingIntervalSeconds
	samplingInterval := time.Duration(mm.settings.XStatsSampleRateSeconds.GetValue() * float64(time.Second))
	samplesToAverage := mm.settings.XStatsSamplesToAverage.GetValue()
	// fmt.Println(samplingInterval, samplesToAverage)

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
			fmt.Println("CLOSING")
			return
		case <-tickChan:
			fmt.Println("TICK")
			time.Sleep(1 * time.Second)
			fmt.Println("current time:", time.Now().Format(time.RFC3339Nano))
			for _, metric := range mm.metrics {
				metric.Sample()
			}
			samplesCollected++
			fmt.Println("samples collected:", samplesCollected)

			if samplesCollected == samplesToAverage {
				aggregatedMetrics := mm.aggregate()
				if len(aggregatedMetrics) > 0 {
					// publish metrics
					// fmt.Println(aggregatedMetrics)
					record := mm.makeStatsRecord(aggregatedMetrics)
					// fmt.Println(record)
					fmt.Println("SENDING RECORD")
					mm.outChan <- record
					fmt.Println("RECORD SENT")
				}
				for _, metric := range mm.metrics {
					metric.Clear()
				}
				// reset samplesCollected
				samplesCollected = int32(0)
			}
			fmt.Println("TICK DONE")
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
	fmt.Println("Stopping metrics monitor")
	mm.cancel()
	fmt.Println("Cancel called")
	mm.wg.Wait()
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

	systemMonitor := &SystemMonitor{
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		settings: settings,
	}

	systemMonitor.assets = []Asset{
		NewMemory(settings, logger, outChan),
	}

	return systemMonitor
}

func (sm *SystemMonitor) Do() {
	sm.logger.Info("Starting system monitor")

	// start monitoring the assets
	for _, asset := range sm.assets {
		asset.Start()
	}

}

func (sm *SystemMonitor) Stop() {
	sm.logger.Info("Stopping system monitor")
	sm.cancel()

	wg := &sync.WaitGroup{}

	for _, asset := range sm.assets {
		wg.Add(1)
		go func(asset Asset) {
			asset.Stop()
			wg.Done()
		}(asset)
	}
	fmt.Println("waiting for all assets to stop")
	wg.Wait()
}
