package server

import (
	"context"
	"testing"

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
