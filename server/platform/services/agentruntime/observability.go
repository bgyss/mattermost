// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime – observability.go
// In-memory per-agent metrics counters (lock-free via sync/atomic).
// Exported via GET /api/v4/agents/metrics.

package agentruntime

import (
	"sync"
	"sync/atomic"
)

// AgentMetrics is a snapshot of runtime counters for a single agent.
type AgentMetrics struct {
	AgentId        string `json:"agent_id"`
	TasksSubmitted int64  `json:"tasks_submitted"`
	TasksCompleted int64  `json:"tasks_completed"`
	TasksFailed    int64  `json:"tasks_failed"`
	TokensTotal    int64  `json:"tokens_total"`
	AvgLatencyMs   int64  `json:"avg_latency_ms"`
}

// agentCounters holds atomic counters for one agent.
type agentCounters struct {
	tasksSubmitted int64
	tasksCompleted int64
	tasksFailed    int64
	tokensTotal    int64
	totalLatencyMs int64
}

// observabilityService collects per-agent metrics in memory.
// It is intentionally simple — no persistence, resets on restart.
// For durable metrics, use the AgentTaskEvents table.
type observabilityService struct {
	mu      sync.RWMutex
	metrics map[string]*agentCounters
}

func newObservabilityService() *observabilityService {
	return &observabilityService{metrics: make(map[string]*agentCounters)}
}

func (o *observabilityService) countersFor(agentId string) *agentCounters {
	o.mu.RLock()
	c, ok := o.metrics[agentId]
	o.mu.RUnlock()
	if ok {
		return c
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	// Re-check after acquiring write lock.
	if c, ok = o.metrics[agentId]; ok {
		return c
	}
	c = &agentCounters{}
	o.metrics[agentId] = c
	return c
}

// RecordSubmit increments the submitted counter for an agent.
func (o *observabilityService) RecordSubmit(agentId string) {
	atomic.AddInt64(&o.countersFor(agentId).tasksSubmitted, 1)
}

// RecordComplete increments completed + accumulates tokens and latency.
func (o *observabilityService) RecordComplete(agentId string, tokens, latencyMs int64) {
	c := o.countersFor(agentId)
	atomic.AddInt64(&c.tasksCompleted, 1)
	atomic.AddInt64(&c.tokensTotal, tokens)
	atomic.AddInt64(&c.totalLatencyMs, latencyMs)
}

// RecordFail increments the failed counter.
func (o *observabilityService) RecordFail(agentId string) {
	atomic.AddInt64(&o.countersFor(agentId).tasksFailed, 1)
}

// GetMetrics returns a snapshot of all per-agent metrics.
func (o *observabilityService) GetMetrics() []AgentMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]AgentMetrics, 0, len(o.metrics))
	for agentId, c := range o.metrics {
		completed := atomic.LoadInt64(&c.tasksCompleted)
		totalLatency := atomic.LoadInt64(&c.totalLatencyMs)
		avgLatency := int64(0)
		if completed > 0 {
			avgLatency = totalLatency / completed
		}
		result = append(result, AgentMetrics{
			AgentId:        agentId,
			TasksSubmitted: atomic.LoadInt64(&c.tasksSubmitted),
			TasksCompleted: completed,
			TasksFailed:    atomic.LoadInt64(&c.tasksFailed),
			TokensTotal:    atomic.LoadInt64(&c.tokensTotal),
			AvgLatencyMs:   avgLatency,
		})
	}
	return result
}
