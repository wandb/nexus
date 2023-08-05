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

func TestHandleRunStart(t *testing.T) {
	ctx := context.Background()
	settings := &service.Settings{}
	// Create a mock logger.
	logger := &observability.NexusLogger{
		Logger: &slog.Logger{},
	}

	// Create a mock system monitor.
	mockSystemMonitor := &monitor.SystemMonitor{
		// You might need to set additional fields or methods to avoid nil pointer dereferences.
		OutChan: make(chan *service.Record, 1), // Example, adjust as needed.
	}

	handler := NewHandler(ctx, settings, logger)
	// Assign the mock system monitor.
	handler.systemMonitor = mockSystemMonitor

	startTime := time.Unix(1000000, 0)
	run := &service.RunRecord{
		StartTime: timestamppb.New(startTime),
	}
	request := &service.RunStartRequest{
		Run: run,
	}
	record := &service.Record{}

	handler.handleRunStart(record, request)

	assert.Equal(t, float64(1000), handler.startTime)
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

func TestHandleRequestDefer(t *testing.T) {
	handler := &Handler{
		recordChan: make(chan *service.Record, 1),
	}

	deferRequest := &service.Request{
		RequestType: &service.Request_Defer{
			Defer: &service.DeferRequest{State: service.DeferRequest_FLUSH_PARTIAL_HISTORY},
		},
	}
	record := &service.Record{
		RecordType: &service.Record_Request{Request: deferRequest},
	}
	handler.historyRecord = &service.HistoryRecord{}

	handler.handleRequest(record)

	receivedRecord := <-handler.recordChan
	assert.NotNil(t, receivedRecord)
	assert.Nil(t, handler.historyRecord)
}

func TestHandleGetSummary(t *testing.T) {
	handler := &Handler{
		consolidatedSummary: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	response := &service.Response{}
	handler.handleGetSummary(nil, response)

	expectedItems := []*service.SummaryItem{
		{Key: "key1", ValueJson: "value1"},
		{Key: "key2", ValueJson: "value2"},
	}

	assert.Equal(t, expectedItems, response.GetGetSummaryResponse().Item)
}

func TestFlushHistory(t *testing.T) {
	handler := &Handler{
		startTime:  1000,
		recordChan: make(chan *service.Record, 2),
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

func TestHandlePartialHistory(t *testing.T) {
	handler := &Handler{
		historyRecord: &service.HistoryRecord{Step: &service.HistoryStep{Num: 1}},
	}

	request := &service.PartialHistoryRequest{
		Step:   &service.HistoryStep{Num: 2},
		Item:   []*service.HistoryItem{{Key: "key1", ValueJson: "value1"}},
		Action: &service.HistoryAction{Flush: true},
	}

	handler.handlePartialHistory(nil, request)

	assert.Equal(t, int32(3), handler.historyRecord.Step.Num)
}
