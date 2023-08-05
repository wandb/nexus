package server

import (
	"context"
	"testing"
	"time"

	"github.com/wandb/wandb/nexus/internal/nexuslib"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	"github.com/wandb/wandb/nexus/pkg/monitor"
	"github.com/wandb/wandb/nexus/pkg/observability"
	"github.com/wandb/wandb/nexus/pkg/service"
)

func TestNewHandler(t *testing.T) {
	ctx := context.Background()
	settings := &service.Settings{}
	logger := &observability.NexusLogger{}

	handler := NewHandler(ctx, settings, logger)

	assert.Equal(t, ctx, handler.ctx)
	assert.Equal(t, settings, handler.settings)
	assert.Equal(t, logger, handler.logger)
	assert.IsType(t, &monitor.SystemMonitor{}, handler.systemMonitor)
	assert.NotNil(t, handler.recordChan)
	assert.NotNil(t, handler.resultChan)
	assert.NotNil(t, handler.consolidatedSummary)
}

func TestSendRecord(t *testing.T) {
	handler := &Handler{
		recordChan: make(chan *service.Record, 1),
	}

	record := &service.Record{}
	handler.sendRecord(record)

	receivedRecord := <-handler.recordChan
	assert.Equal(t, record, receivedRecord)
}

func TestClose(t *testing.T) {
	handler := &Handler{
		resultChan: make(chan *service.Result, 1),
		recordChan: make(chan *service.Record, 1),
	}

	handler.close()

	_, ok := <-handler.resultChan
	assert.False(t, ok)
	_, ok = <-handler.recordChan
	assert.False(t, ok)
}

func TestGetRun(t *testing.T) {
	runRecord := &service.RunRecord{}
	handler := &Handler{
		runRecord: runRecord,
	}

	returnedRunRecord := handler.GetRun()
	assert.Equal(t, runRecord, returnedRunRecord)
}

// Create a mock version of SystemMonitor
type MockSystemMonitor struct{}

func (sm *MockSystemMonitor) Do()                              {}
func (sm *MockSystemMonitor) Monitor(asset monitor.Asset)      {}
func (sm *MockSystemMonitor) GetOutChan() chan *service.Record { return nil }
func (sm *MockSystemMonitor) Stop()                            {}

func TestHandleRunStart(t *testing.T) {
	ctx := context.Background()
	settings := &service.Settings{}
	// Create a mock logger.
	logger := &observability.NexusLogger{
		Logger: &slog.Logger{},
	}

	handler := NewHandler(ctx, settings, logger)
	handler.systemMonitor = &MockSystemMonitor{}

	startTime := time.Unix(1000000, 0)
	run := &service.RunRecord{
		StartTime: timestamppb.New(startTime),
	}
	request := &service.RunStartRequest{
		Run: run,
	}
	record := &service.Record{}

	handler.handleRunStart(record, request)

	assert.Equal(t, float64(1000000), handler.startTime)
	assert.Equal(t, run, handler.runRecord)
}

func TestHandleAttach(t *testing.T) {
	runRecord := &service.RunRecord{}
	handler := &Handler{
		runRecord: runRecord,
	}

	response := &service.Response{}
	handler.handleAttach(nil, response)

	assert.Equal(t, &service.AttachResponse{Run: runRecord}, response.GetAttachResponse())
}

func TestHandleSummary(t *testing.T) {
	handler := &Handler{
		recordChan:          make(chan *service.Record, 1),
		consolidatedSummary: make(map[string]string),
	}

	update := []*service.SummaryItem{
		{Key: "key1", ValueJson: "value1"},
		{Key: "key2", ValueJson: "value2"},
	}
	summary := &service.SummaryRecord{Update: update}
	record := &service.Record{}
	handler.handleSummary(record, summary)

	summaryRecord := <-handler.recordChan
	expectedSummary := nexuslib.ConsolidateSummaryItems(handler.consolidatedSummary, update)
	assert.Equal(t, expectedSummary, summaryRecord)
}

func TestFlushHistory(t *testing.T) {
	handler := &Handler{
		startTime:           1000,
		recordChan:          make(chan *service.Record, 2),
		consolidatedSummary: make(map[string]string),
	}

	history := &service.HistoryRecord{
		Item: []*service.HistoryItem{
			{Key: "_timestamp", ValueJson: "2000"},
		},
		Step: &service.HistoryStep{Num: 1},
	}

	handler.flushHistory(history)

	summaryRecord := <-handler.recordChan
	historyRecord := <-handler.recordChan

	assert.Equal(t, "_runtime", summaryRecord.GetSummary().Update[0].Key)
	assert.Equal(t, "1.000000", summaryRecord.GetSummary().Update[0].ValueJson)
	assert.Equal(t, "_step", summaryRecord.GetSummary().Update[1].Key)
	assert.Equal(t, "1", summaryRecord.GetSummary().Update[1].ValueJson)
	assert.Equal(t, history, historyRecord.GetHistory())
}