// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// agentCtx returns a context with agentId and taskId injected.
func agentCtx(agentId, taskId string) context.Context {
	ctx := context.WithValue(context.Background(), contextKeyAgentID, agentId)
	return context.WithValue(ctx, contextKeyTaskID, taskId)
}

func newMemoryToolsFixture(t *testing.T) (*MemoryManager, *mocks.AgentStore, request.CTX) {
	t.Helper()
	mockAgentStore := mocks.NewAgentStore(t)
	mockStore := mocks.NewStore(t)
	mockStore.On("Agent").Return(mockAgentStore).Maybe()
	logger := mlog.CreateConsoleTestLogger(t)
	mem := newMemoryManager(mockStore, logger)
	rctx := request.EmptyContext(logger)
	return mem, mockAgentStore, rctx
}

// ---------------------------------------------------------------------------
// remember tool
// ---------------------------------------------------------------------------

func TestRememberTool_Execute_Success(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &rememberTool{mem: mem}

	agentStore.On("SaveAgentMemory", mock.MatchedBy(func(m *model.AgentMemory) bool {
		return m.AgentId == "agentA" && m.Key == "color" && m.ValueText == "blue" &&
			m.Scope == model.AgentMemoryScopeGlobal
	})).Return(&model.AgentMemory{}, nil)

	args, _ := json.Marshal(map[string]string{"key": "color", "value": "blue"})
	result, err := tool.Execute(agentCtx("agentA", "task1"), rctx, args)

	require.NoError(t, err)
	assert.Contains(t, result, "color")
	assert.Contains(t, result, "blue")
}

func TestRememberTool_Execute_MissingKey(t *testing.T) {
	mem, _, rctx := newMemoryToolsFixture(t)
	tool := &rememberTool{mem: mem}

	args, _ := json.Marshal(map[string]string{"value": "blue"})
	_, err := tool.Execute(agentCtx("agentA", "task1"), rctx, args)
	require.Error(t, err)
}

func TestRememberTool_Execute_NoAgentIDInContext(t *testing.T) {
	mem, _, rctx := newMemoryToolsFixture(t)
	tool := &rememberTool{mem: mem}

	args, _ := json.Marshal(map[string]string{"key": "k", "value": "v"})
	_, err := tool.Execute(context.Background(), rctx, args) // no agent id in ctx
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent ID")
}

func TestRememberTool_Execute_TaskScopeDefaultsScopeId(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &rememberTool{mem: mem}

	agentStore.On("SaveAgentMemory", mock.MatchedBy(func(m *model.AgentMemory) bool {
		return m.Scope == model.AgentMemoryScopeTask && m.ScopeId == "task99"
	})).Return(&model.AgentMemory{}, nil)

	args, _ := json.Marshal(map[string]string{"key": "k", "value": "v", "scope": "task"})
	_, err := tool.Execute(agentCtx("agentA", "task99"), rctx, args)
	require.NoError(t, err)
	agentStore.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// recall tool
// ---------------------------------------------------------------------------

func TestRecallTool_Execute_Found(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &recallTool{mem: mem}

	agentStore.On("GetAgentMemory", "agentA", model.AgentMemoryScopeGlobal, "", "language").
		Return(&model.AgentMemory{ValueText: "Go", ExpireAt: 0}, nil)

	args, _ := json.Marshal(map[string]string{"key": "language"})
	result, err := tool.Execute(agentCtx("agentA", "t1"), rctx, args)
	require.NoError(t, err)
	assert.Equal(t, "Go", result)
}

func TestRecallTool_Execute_NotFound(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &recallTool{mem: mem}

	agentStore.On("GetAgentMemory", "agentA", model.AgentMemoryScopeGlobal, "", "missing").
		Return(nil, store.NewErrNotFound("AgentMemory", "missing"))

	args, _ := json.Marshal(map[string]string{"key": "missing"})
	result, err := tool.Execute(agentCtx("agentA", "t1"), rctx, args)
	require.NoError(t, err, "not-found should not propagate as error")
	assert.Contains(t, result, "missing")
}

func TestRecallTool_Execute_NoAgentID(t *testing.T) {
	mem, _, rctx := newMemoryToolsFixture(t)
	tool := &recallTool{mem: mem}

	args, _ := json.Marshal(map[string]string{"key": "k"})
	_, err := tool.Execute(context.Background(), rctx, args)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// recall_all tool
// ---------------------------------------------------------------------------

func TestRecallAllTool_Execute_WithEntries(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &recallAllTool{mem: mem}

	agentStore.On("GetAgentMemoryByScope", "agentA", model.AgentMemoryScopeGlobal, "").
		Return([]*model.AgentMemory{
			{Key: "a", ValueText: "1", ExpireAt: 0},
			{Key: "b", ValueText: "2", ExpireAt: 0},
		}, nil)

	args, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(agentCtx("agentA", "t1"), rctx, args)
	require.NoError(t, err)
	assert.Contains(t, result, "a: 1")
	assert.Contains(t, result, "b: 2")
}

func TestRecallAllTool_Execute_Empty(t *testing.T) {
	mem, agentStore, rctx := newMemoryToolsFixture(t)
	tool := &recallAllTool{mem: mem}

	agentStore.On("GetAgentMemoryByScope", "agentA", model.AgentMemoryScopeGlobal, "").
		Return(nil, nil)

	args, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(agentCtx("agentA", "t1"), rctx, args)
	require.NoError(t, err)
	assert.Contains(t, result, "No memories")
}

func TestRecallAllTool_Definition(t *testing.T) {
	mem, _, _ := newMemoryToolsFixture(t)
	tool := &recallAllTool{mem: mem}
	def := tool.Definition()
	assert.Equal(t, "recall_all", def.Name)
}
