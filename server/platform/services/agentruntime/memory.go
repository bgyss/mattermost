// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime – memory.go
// Three-tier KV memory manager (global / task / user scope).
// Memories are injected into the LLM system prompt at context-build time.

package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const (
	memoryExpiryInterval = time.Hour
	// maxMemoryInject caps how many global memories are prepended to the system prompt.
	maxMemoryInject = 20
)

// MemoryManager provides KV memory operations for agents backed by the AgentStore.
type MemoryManager struct {
	store store.Store
	log   *mlog.Logger
}

func newMemoryManager(st store.Store, log *mlog.Logger) *MemoryManager {
	return &MemoryManager{store: st, log: log}
}

// Remember stores (or updates) a fact for an agent.
// scope is one of AgentMemoryScopeGlobal / AgentMemoryScopeTask / AgentMemoryScopeUser.
// ttlMs ≤ 0 means no expiry.
func (m *MemoryManager) Remember(agentId, scope, scopeId, key, value string, ttlMs int64) error {
	if scope == "" {
		scope = model.AgentMemoryScopeGlobal
	}
	mem := &model.AgentMemory{
		AgentId:   agentId,
		Scope:     scope,
		ScopeId:   scopeId,
		Key:       key,
		ValueText: value,
	}
	if ttlMs > 0 {
		mem.ExpireAt = model.GetMillis() + ttlMs
	}
	// SaveAgentMemory uses ON CONFLICT DO UPDATE — safe to call for both insert and update.
	if _, err := m.store.Agent().SaveAgentMemory(mem); err != nil {
		return fmt.Errorf("memory: remember: %w", err)
	}
	return nil
}

// Recall returns a single memory entry by key, respecting expiry.
func (m *MemoryManager) Recall(agentId, scope, scopeId, key string) (*model.AgentMemory, error) {
	if scope == "" {
		scope = model.AgentMemoryScopeGlobal
	}
	mem, err := m.store.Agent().GetAgentMemory(agentId, scope, scopeId, key)
	if err != nil {
		return nil, err
	}
	// Belt-and-suspenders expiry check; bulk cleanup runs hourly.
	if mem.ExpireAt > 0 && mem.ExpireAt < model.GetMillis() {
		return nil, store.NewErrNotFound("AgentMemory", key)
	}
	return mem, nil
}

// RecallAll lists all non-expired memory entries for an agent in a scope.
func (m *MemoryManager) RecallAll(agentId, scope, scopeId string) ([]*model.AgentMemory, error) {
	if scope == "" {
		scope = model.AgentMemoryScopeGlobal
	}
	mems, err := m.store.Agent().GetAgentMemoryByScope(agentId, scope, scopeId)
	if err != nil {
		return nil, err
	}
	now := model.GetMillis()
	var valid []*model.AgentMemory
	for _, mem := range mems {
		if mem.ExpireAt == 0 || mem.ExpireAt > now {
			valid = append(valid, mem)
		}
	}
	return valid, nil
}

// BuildMemoryContext returns a formatted string of the agent's global memories
// suitable for appending to the system prompt.
func (m *MemoryManager) BuildMemoryContext(agentId string) string {
	mems, err := m.RecallAll(agentId, model.AgentMemoryScopeGlobal, "")
	if err != nil || len(mems) == 0 {
		return ""
	}
	limit := len(mems)
	if limit > maxMemoryInject {
		limit = maxMemoryInject
	}
	var sb strings.Builder
	sb.WriteString("\n\n## Your Memory\nYou have stored the following facts. Use them when relevant:\n")
	for _, mem := range mems[:limit] {
		fmt.Fprintf(&sb, "- **%s**: %s\n", mem.Key, mem.ValueText)
	}
	return sb.String()
}

// runExpiryCleanup runs until ctx is cancelled, periodically deleting expired entries.
func (m *MemoryManager) runExpiryCleanup(ctx context.Context) {
	ticker := time.NewTicker(memoryExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.store.Agent().DeleteExpiredAgentMemory(); err != nil {
				m.log.Warn("memory: expiry cleanup failed", mlog.Err(err))
			}
		}
	}
}
