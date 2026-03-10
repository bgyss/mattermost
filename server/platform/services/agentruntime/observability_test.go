// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityService_RecordSubmit(t *testing.T) {
	obs := newObservabilityService()

	obs.RecordSubmit("agent1")
	obs.RecordSubmit("agent1")
	obs.RecordSubmit("agent2")

	metrics := obs.GetMetrics()
	require.Len(t, metrics, 2)

	byAgent := make(map[string]AgentMetrics)
	for _, m := range metrics {
		byAgent[m.AgentId] = m
	}

	assert.Equal(t, int64(2), byAgent["agent1"].TasksSubmitted)
	assert.Equal(t, int64(1), byAgent["agent2"].TasksSubmitted)
}

func TestObservabilityService_RecordComplete(t *testing.T) {
	obs := newObservabilityService()

	obs.RecordComplete("agent1", 1000, 500)
	obs.RecordComplete("agent1", 2000, 1000)

	metrics := obs.GetMetrics()
	require.Len(t, metrics, 1)
	m := metrics[0]

	assert.Equal(t, int64(2), m.TasksCompleted)
	assert.Equal(t, int64(3000), m.TokensTotal)
	assert.Equal(t, int64(750), m.AvgLatencyMs) // (500+1000)/2
}

func TestObservabilityService_RecordFail(t *testing.T) {
	obs := newObservabilityService()

	obs.RecordFail("agent1")
	obs.RecordFail("agent1")

	metrics := obs.GetMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, int64(2), metrics[0].TasksFailed)
}

func TestObservabilityService_AvgLatencyZeroWhenNoCompleted(t *testing.T) {
	obs := newObservabilityService()
	obs.RecordSubmit("agent1")

	metrics := obs.GetMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, int64(0), metrics[0].AvgLatencyMs)
}

func TestObservabilityService_GetMetricsEmpty(t *testing.T) {
	obs := newObservabilityService()
	assert.Empty(t, obs.GetMetrics())
}

func TestObservabilityService_ConcurrentAccess(t *testing.T) {
	obs := newObservabilityService()
	const goroutines = 50
	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			agentId := "agent1"
			if i%2 == 0 {
				agentId = "agent2"
			}
			obs.RecordSubmit(agentId)
			obs.RecordComplete(agentId, 100, 200)
		}(i)
	}
	wg.Wait()

	metrics := obs.GetMetrics()
	require.Len(t, metrics, 2)

	byAgent := make(map[string]AgentMetrics)
	for _, m := range metrics {
		byAgent[m.AgentId] = m
	}
	assert.Equal(t, int64(goroutines/2), byAgent["agent1"].TasksSubmitted)
	assert.Equal(t, int64(goroutines/2), byAgent["agent2"].TasksSubmitted)
}

func TestObservabilityService_MultipleAgentsIsolated(t *testing.T) {
	obs := newObservabilityService()

	obs.RecordSubmit("a")
	obs.RecordSubmit("b")
	obs.RecordComplete("a", 500, 100)
	obs.RecordFail("b")

	byAgent := make(map[string]AgentMetrics)
	for _, m := range obs.GetMetrics() {
		byAgent[m.AgentId] = m
	}

	assert.Equal(t, int64(1), byAgent["a"].TasksCompleted)
	assert.Equal(t, int64(0), byAgent["a"].TasksFailed)
	assert.Equal(t, int64(0), byAgent["b"].TasksCompleted)
	assert.Equal(t, int64(1), byAgent["b"].TasksFailed)
}
