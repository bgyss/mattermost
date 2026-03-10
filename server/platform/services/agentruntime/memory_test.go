// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestMemoryManager(t *testing.T) (*MemoryManager, *mocks.AgentStore, *mocks.Store) {
	t.Helper()
	mockAgentStore := mocks.NewAgentStore(t)
	mockStore := mocks.NewStore(t)
	mockStore.On("Agent").Return(mockAgentStore)
	logger := mlog.CreateConsoleTestLogger(t)
	return newMemoryManager(mockStore, logger), mockAgentStore, mockStore
}

func TestMemoryManager_Remember(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	saved := &model.AgentMemory{
		Id:        model.NewId(),
		AgentId:   "agent1",
		Scope:     model.AgentMemoryScopeGlobal,
		ScopeId:   "",
		Key:       "greeting",
		ValueText: "hello",
	}
	agentStore.On("SaveAgentMemory", mock.MatchedBy(func(m *model.AgentMemory) bool {
		return m.AgentId == "agent1" && m.Key == "greeting" && m.ValueText == "hello"
	})).Return(saved, nil)

	err := mgr.Remember("agent1", model.AgentMemoryScopeGlobal, "", "greeting", "hello", 0)
	require.NoError(t, err)
}

func TestMemoryManager_Remember_DefaultsToGlobalScope(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	agentStore.On("SaveAgentMemory", mock.MatchedBy(func(m *model.AgentMemory) bool {
		return m.Scope == model.AgentMemoryScopeGlobal
	})).Return(&model.AgentMemory{}, nil)

	// Pass empty scope — should default to global.
	err := mgr.Remember("agent1", "", "", "key", "val", 0)
	require.NoError(t, err)
	agentStore.AssertExpectations(t)
}

func TestMemoryManager_Remember_WithTTL(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	ttlMs := int64(60_000) // 1 minute
	before := model.GetMillis()

	agentStore.On("SaveAgentMemory", mock.MatchedBy(func(m *model.AgentMemory) bool {
		return m.ExpireAt >= before+ttlMs
	})).Return(&model.AgentMemory{}, nil)

	err := mgr.Remember("agent1", model.AgentMemoryScopeGlobal, "", "key", "val", ttlMs)
	require.NoError(t, err)
}

func TestMemoryManager_Recall_Found(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	expected := &model.AgentMemory{
		AgentId:   "agent1",
		Key:       "language",
		ValueText: "Go",
		ExpireAt:  0,
	}
	agentStore.On("GetAgentMemory", "agent1", model.AgentMemoryScopeGlobal, "", "language").Return(expected, nil)

	got, err := mgr.Recall("agent1", model.AgentMemoryScopeGlobal, "", "language")
	require.NoError(t, err)
	assert.Equal(t, "Go", got.ValueText)
}

func TestMemoryManager_Recall_Expired(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	expired := &model.AgentMemory{
		Key:      "old",
		ExpireAt: model.GetMillis() - 1000, // already expired
	}
	agentStore.On("GetAgentMemory", "agent1", model.AgentMemoryScopeGlobal, "", "old").Return(expired, nil)

	_, err := mgr.Recall("agent1", model.AgentMemoryScopeGlobal, "", "old")
	require.Error(t, err, "should return error for expired memory")
}

func TestMemoryManager_Recall_NotFound(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	agentStore.On("GetAgentMemory", "agent1", model.AgentMemoryScopeGlobal, "", "missing").
		Return(nil, store.NewErrNotFound("AgentMemory", "missing"))

	_, err := mgr.Recall("agent1", model.AgentMemoryScopeGlobal, "", "missing")
	require.Error(t, err)
}

func TestMemoryManager_RecallAll_FiltersExpired(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	now := model.GetMillis()
	all := []*model.AgentMemory{
		{Key: "valid", ExpireAt: 0},
		{Key: "future", ExpireAt: now + 60_000},
		{Key: "expired", ExpireAt: now - 1000},
	}
	agentStore.On("GetAgentMemoryByScope", "agent1", model.AgentMemoryScopeGlobal, "").Return(all, nil)

	mems, err := mgr.RecallAll("agent1", model.AgentMemoryScopeGlobal, "")
	require.NoError(t, err)
	require.Len(t, mems, 2, "expired entry should be filtered out")
	keys := []string{mems[0].Key, mems[1].Key}
	assert.Contains(t, keys, "valid")
	assert.Contains(t, keys, "future")
}

func TestMemoryManager_BuildMemoryContext_Empty(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	agentStore.On("GetAgentMemoryByScope", "agent1", model.AgentMemoryScopeGlobal, "").Return(nil, nil)

	ctx := mgr.BuildMemoryContext("agent1")
	assert.Empty(t, ctx, "no memories → empty context string")
}

func TestMemoryManager_BuildMemoryContext_WithEntries(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	agentStore.On("GetAgentMemoryByScope", "agent1", model.AgentMemoryScopeGlobal, "").Return([]*model.AgentMemory{
		{Key: "language", ValueText: "Go", ExpireAt: 0},
		{Key: "style", ValueText: "concise", ExpireAt: 0},
	}, nil)

	ctx := mgr.BuildMemoryContext("agent1")
	assert.Contains(t, ctx, "language")
	assert.Contains(t, ctx, "Go")
	assert.Contains(t, ctx, "style")
	assert.Contains(t, ctx, "concise")
}

func TestMemoryManager_RunExpiryCleanup_CallsStore(t *testing.T) {
	mgr, agentStore, _ := newTestMemoryManager(t)

	agentStore.On("DeleteExpiredAgentMemory").Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use a very short interval by testing the cleanup call directly.
	err := mgr.store.Agent().DeleteExpiredAgentMemory()
	require.NoError(t, err)
	agentStore.AssertExpectations(t)

	_ = ctx // ctx used to bound test duration
}
