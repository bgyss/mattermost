// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentStore(t *testing.T) {
	StoreTest(t, func(t *testing.T, _ request.CTX, ss store.Store) {
		t.Run("AgentDefinition_SaveAndGet", testAgentDefinitionSaveAndGet(ss))
		t.Run("AgentDefinition_Update", testAgentDefinitionUpdate(ss))
		t.Run("AgentDefinition_Delete", testAgentDefinitionDelete(ss))
		t.Run("AgentDefinition_GetByBotUserId", testAgentDefinitionGetByBotUserId(ss))
		t.Run("AgentDefinition_GetByWorkgroup", testAgentDefinitionGetByWorkgroup(ss))
		t.Run("AgentTask_SaveAndGet", testAgentTaskSaveAndGet(ss))
		t.Run("AgentTask_Update", testAgentTaskUpdate(ss))
		t.Run("AgentTask_GetTree", testAgentTaskGetTree(ss))
		t.Run("AgentTask_GetActive", testAgentTaskGetActive(ss))
		t.Run("AgentTask_ClaimPending", testAgentTaskClaimPending(ss))
		t.Run("AgentMemory_SaveAndGet", testAgentMemorySaveAndGet(ss))
		t.Run("AgentMemory_Upsert", testAgentMemoryUpsert(ss))
		t.Run("AgentMemory_GetByScope", testAgentMemoryGetByScope(ss))
		t.Run("AgentMemory_Delete", testAgentMemoryDelete(ss))
		t.Run("AgentMemory_DeleteExpired", testAgentMemoryDeleteExpired(ss))
		t.Run("AgentTaskEvent_SaveAndGet", testAgentTaskEventSaveAndGet(ss))
	})
}

// ---------------------------------------------------------------------------
// Helper: minimal valid AgentDefinition
// ---------------------------------------------------------------------------

func makeAgentDef(workgroupId string) *model.AgentDefinition {
	return &model.AgentDefinition{
		WorkgroupId:  workgroupId,
		Role:         "specialist",
		BotUserId:    model.NewId(),
		DisplayName:  "Test Agent",
		SystemPrompt: "You are a test agent.",
		LLMServiceId: "openclaw",
		ModelId:      "claude-sonnet-4-6",
		Capabilities: []string{"testing"},
		Tools:        []string{},
	}
}

// ---------------------------------------------------------------------------
// AgentDefinition tests
// ---------------------------------------------------------------------------

func testAgentDefinitionSaveAndGet(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		def := makeAgentDef(model.NewId())
		saved, err := ss.Agent().SaveAgentDefinition(def)
		require.NoError(t, err)
		require.NotEmpty(t, saved.Id)
		assert.Equal(t, def.DisplayName, saved.DisplayName)
		assert.Equal(t, def.BotUserId, saved.BotUserId)

		got, err := ss.Agent().GetAgentDefinition(saved.Id)
		require.NoError(t, err)
		assert.Equal(t, saved.Id, got.Id)
		assert.Equal(t, saved.SystemPrompt, got.SystemPrompt)
		assert.Equal(t, []string{"testing"}, got.Capabilities)
	}
}

func testAgentDefinitionUpdate(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		def := makeAgentDef(model.NewId())
		saved, err := ss.Agent().SaveAgentDefinition(def)
		require.NoError(t, err)

		saved.DisplayName = "Updated Name"
		saved.Capabilities = []string{"cap_a", "cap_b"}
		updated, err := ss.Agent().UpdateAgentDefinition(saved)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.DisplayName)
		assert.Equal(t, []string{"cap_a", "cap_b"}, updated.Capabilities)
	}
}

func testAgentDefinitionDelete(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		def := makeAgentDef(model.NewId())
		saved, err := ss.Agent().SaveAgentDefinition(def)
		require.NoError(t, err)

		err = ss.Agent().DeleteAgentDefinition(saved.Id)
		require.NoError(t, err)

		_, err = ss.Agent().GetAgentDefinition(saved.Id)
		require.Error(t, err, "should not find deleted definition")
	}
}

func testAgentDefinitionGetByBotUserId(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		botId := model.NewId()
		def := makeAgentDef(model.NewId())
		def.BotUserId = botId

		saved, err := ss.Agent().SaveAgentDefinition(def)
		require.NoError(t, err)

		got, err := ss.Agent().GetAgentDefinitionByBotUserId(botId)
		require.NoError(t, err)
		assert.Equal(t, saved.Id, got.Id)

		// Non-existent bot user should return not-found
		_, err = ss.Agent().GetAgentDefinitionByBotUserId(model.NewId())
		require.Error(t, err)
	}
}

func testAgentDefinitionGetByWorkgroup(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		wgId := model.NewId()
		for range 3 {
			_, err := ss.Agent().SaveAgentDefinition(makeAgentDef(wgId))
			require.NoError(t, err)
		}
		// One agent in a different workgroup
		_, err := ss.Agent().SaveAgentDefinition(makeAgentDef(model.NewId()))
		require.NoError(t, err)

		defs, err := ss.Agent().GetAgentDefinitionsByWorkgroup(wgId)
		require.NoError(t, err)
		assert.Len(t, defs, 3)
	}
}

// ---------------------------------------------------------------------------
// AgentTask tests
// ---------------------------------------------------------------------------

func makeAgentTask(agentId, channelId string) *model.AgentTask {
	return &model.AgentTask{
		AgentId:   agentId,
		ChannelId: channelId,
		Input:     model.AgentTaskInput{Text: "test input"},
		Priority:  model.AgentTaskPriorityNormal,
	}
}

func testAgentTaskSaveAndGet(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		task := makeAgentTask(model.NewId(), model.NewId())
		saved, err := ss.Agent().SaveAgentTask(task)
		require.NoError(t, err)
		require.NotEmpty(t, saved.Id)
		assert.Equal(t, model.AgentTaskStatusPending, saved.Status)
		assert.Equal(t, "test input", saved.Input.Text)
		assert.Equal(t, saved.Id, saved.RootTaskId)

		got, err := ss.Agent().GetAgentTask(saved.Id)
		require.NoError(t, err)
		assert.Equal(t, saved.Id, got.Id)
		assert.Equal(t, model.AgentTaskPriorityNormal, got.Priority)
	}
}

func testAgentTaskUpdate(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		task := makeAgentTask(model.NewId(), model.NewId())
		saved, err := ss.Agent().SaveAgentTask(task)
		require.NoError(t, err)

		saved.Status = model.AgentTaskStatusRunning
		saved.TokensUsed = 500
		updated, err := ss.Agent().UpdateAgentTask(saved)
		require.NoError(t, err)
		assert.Equal(t, model.AgentTaskStatusRunning, updated.Status)
		assert.Equal(t, 500, updated.TokensUsed)
	}
}

func testAgentTaskGetTree(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		channelId := model.NewId()

		// Root task
		root := makeAgentTask(agentId, channelId)
		root, err := ss.Agent().SaveAgentTask(root)
		require.NoError(t, err)

		// Child tasks
		for range 2 {
			child := makeAgentTask(agentId, channelId)
			child.ParentTaskId = root.Id
			child.RootTaskId = root.Id
			_, err := ss.Agent().SaveAgentTask(child)
			require.NoError(t, err)
		}

		tree, err := ss.Agent().GetTaskTree(root.Id)
		require.NoError(t, err)
		assert.Len(t, tree, 3, "root + 2 children")
	}
}

func testAgentTaskGetActive(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		channelId := model.NewId()

		running := makeAgentTask(agentId, channelId)
		running, err := ss.Agent().SaveAgentTask(running)
		require.NoError(t, err)
		running.Status = model.AgentTaskStatusRunning
		_, err = ss.Agent().UpdateAgentTask(running)
		require.NoError(t, err)

		complete := makeAgentTask(agentId, channelId)
		complete, err = ss.Agent().SaveAgentTask(complete)
		require.NoError(t, err)
		complete.Status = model.AgentTaskStatusComplete
		_, err = ss.Agent().UpdateAgentTask(complete)
		require.NoError(t, err)

		active, err := ss.Agent().GetActiveTasksForAgent(agentId)
		require.NoError(t, err)
		assert.Len(t, active, 1, "only running task should appear")
		assert.Equal(t, running.Id, active[0].Id)
	}
}

func testAgentTaskClaimPending(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		task := makeAgentTask(agentId, model.NewId())
		saved, err := ss.Agent().SaveAgentTask(task)
		require.NoError(t, err)
		assert.Equal(t, model.AgentTaskStatusPending, saved.Status)

		serverId := model.NewId()
		claimed, err := ss.Agent().ClaimPendingTask(agentId, serverId)
		require.NoError(t, err)
		require.NotNil(t, claimed)
		assert.Equal(t, saved.Id, claimed.Id)
		assert.Equal(t, serverId, claimed.ClaimedByServer)

		// Second claim attempt should return nil (already claimed).
		claimed2, err := ss.Agent().ClaimPendingTask(agentId, serverId)
		require.NoError(t, err)
		assert.Nil(t, claimed2)
	}
}

// ---------------------------------------------------------------------------
// AgentMemory tests
// ---------------------------------------------------------------------------

func testAgentMemorySaveAndGet(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		mem := &model.AgentMemory{
			AgentId:   model.NewId(),
			Scope:     model.AgentMemoryScopeGlobal,
			ScopeId:   "",
			Key:       "language",
			ValueText: "Go",
		}
		saved, err := ss.Agent().SaveAgentMemory(mem)
		require.NoError(t, err)
		require.NotEmpty(t, saved.Id)

		got, err := ss.Agent().GetAgentMemory(mem.AgentId, model.AgentMemoryScopeGlobal, "", "language")
		require.NoError(t, err)
		assert.Equal(t, "Go", got.ValueText)
	}
}

func testAgentMemoryUpsert(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		mem := &model.AgentMemory{
			AgentId:   agentId,
			Scope:     model.AgentMemoryScopeGlobal,
			Key:       "pref",
			ValueText: "v1",
		}
		_, err := ss.Agent().SaveAgentMemory(mem)
		require.NoError(t, err)

		// Upsert with same key should update.
		mem2 := &model.AgentMemory{
			AgentId:   agentId,
			Scope:     model.AgentMemoryScopeGlobal,
			Key:       "pref",
			ValueText: "v2",
		}
		_, err = ss.Agent().SaveAgentMemory(mem2)
		require.NoError(t, err)

		got, err := ss.Agent().GetAgentMemory(agentId, model.AgentMemoryScopeGlobal, "", "pref")
		require.NoError(t, err)
		assert.Equal(t, "v2", got.ValueText, "upsert should update value")
	}
}

func testAgentMemoryGetByScope(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		for _, key := range []string{"k1", "k2", "k3"} {
			_, err := ss.Agent().SaveAgentMemory(&model.AgentMemory{
				AgentId:   agentId,
				Scope:     model.AgentMemoryScopeGlobal,
				Key:       key,
				ValueText: "val",
			})
			require.NoError(t, err)
		}

		mems, err := ss.Agent().GetAgentMemoryByScope(agentId, model.AgentMemoryScopeGlobal, "")
		require.NoError(t, err)
		assert.Len(t, mems, 3)
	}
}

func testAgentMemoryDelete(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		_, err := ss.Agent().SaveAgentMemory(&model.AgentMemory{
			AgentId:   agentId,
			Scope:     model.AgentMemoryScopeGlobal,
			Key:       "k",
			ValueText: "v",
		})
		require.NoError(t, err)

		err = ss.Agent().DeleteAgentMemory(agentId, model.AgentMemoryScopeGlobal, "")
		require.NoError(t, err)

		mems, err := ss.Agent().GetAgentMemoryByScope(agentId, model.AgentMemoryScopeGlobal, "")
		require.NoError(t, err)
		assert.Empty(t, mems)
	}
}

func testAgentMemoryDeleteExpired(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()

		// Expired entry
		_, err := ss.Agent().SaveAgentMemory(&model.AgentMemory{
			AgentId:   agentId,
			Scope:     model.AgentMemoryScopeGlobal,
			Key:       "expired",
			ValueText: "old",
			ExpireAt:  1, // epoch ms — definitely in the past
		})
		require.NoError(t, err)

		// Valid entry
		_, err = ss.Agent().SaveAgentMemory(&model.AgentMemory{
			AgentId:   agentId,
			Scope:     model.AgentMemoryScopeGlobal,
			Key:       "valid",
			ValueText: "new",
			ExpireAt:  0,
		})
		require.NoError(t, err)

		err = ss.Agent().DeleteExpiredAgentMemory()
		require.NoError(t, err)

		mems, err := ss.Agent().GetAgentMemoryByScope(agentId, model.AgentMemoryScopeGlobal, "")
		require.NoError(t, err)
		require.Len(t, mems, 1)
		assert.Equal(t, "valid", mems[0].Key)
	}
}

// ---------------------------------------------------------------------------
// AgentTaskEvent tests
// ---------------------------------------------------------------------------

func testAgentTaskEventSaveAndGet(ss store.Store) func(*testing.T) {
	return func(t *testing.T) {
		agentId := model.NewId()
		task := makeAgentTask(agentId, model.NewId())
		saved, err := ss.Agent().SaveAgentTask(task)
		require.NoError(t, err)

		evt := &model.AgentTaskEvent{
			TaskId:    saved.Id,
			AgentId:   agentId,
			EventType: model.AgentTaskEventTypeToolCall,
			Payload:   map[string]interface{}{"tool": "search_posts"},
		}
		savedEvt, err := ss.Agent().SaveAgentTaskEvent(evt)
		require.NoError(t, err)
		require.NotEmpty(t, savedEvt.Id)

		events, err := ss.Agent().GetAgentTaskEvents(saved.Id, 0, 10)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, model.AgentTaskEventTypeToolCall, events[0].EventType)
		assert.Equal(t, "search_posts", events[0].Payload["tool"])
	}
}
