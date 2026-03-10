// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTask_PreSave(t *testing.T) {
	t.Run("generates IDs when empty", func(t *testing.T) {
		task := &AgentTask{
			AgentId:   NewId(),
			ChannelId: NewId(),
		}
		task.PreSave()

		require.NotEmpty(t, task.Id)
		require.NotEmpty(t, task.RootTaskId)
		assert.Equal(t, task.Id, task.RootTaskId, "root task should be its own root")
		assert.Equal(t, AgentTaskStatusPending, task.Status)
		assert.Greater(t, task.CreateAt, int64(0))
		assert.Greater(t, task.UpdateAt, int64(0))
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		id := NewId()
		task := &AgentTask{Id: id, AgentId: NewId(), ChannelId: NewId()}
		task.PreSave()
		assert.Equal(t, id, task.Id)
	})

	t.Run("preserves explicit root task ID for subtasks", func(t *testing.T) {
		rootId := NewId()
		task := &AgentTask{
			AgentId:      NewId(),
			ChannelId:    NewId(),
			ParentTaskId: NewId(),
			RootTaskId:   rootId,
		}
		task.PreSave()
		assert.Equal(t, rootId, task.RootTaskId)
	})

	t.Run("sets default priority", func(t *testing.T) {
		task := &AgentTask{AgentId: NewId(), ChannelId: NewId()}
		task.PreSave()
		assert.Equal(t, AgentTaskPriorityNormal, task.Priority)
	})

	t.Run("preserves non-zero priority", func(t *testing.T) {
		task := &AgentTask{
			AgentId:   NewId(),
			ChannelId: NewId(),
			Priority:  AgentTaskPriorityHigh,
		}
		task.PreSave()
		assert.Equal(t, AgentTaskPriorityHigh, task.Priority)
	})
}

func TestAgentTaskStatus_Constants(t *testing.T) {
	// Verify status constant values are stable (string comparisons in DB/WS).
	assert.Equal(t, "pending", AgentTaskStatusPending)
	assert.Equal(t, "claimed", AgentTaskStatusClaimed)
	assert.Equal(t, "running", AgentTaskStatusRunning)
	assert.Equal(t, "awaiting_subtask", AgentTaskStatusAwaitingSubtask)
	assert.Equal(t, "complete", AgentTaskStatusComplete)
	assert.Equal(t, "failed", AgentTaskStatusFailed)
}

func TestAgentTaskPriority_Constants(t *testing.T) {
	assert.Greater(t, AgentTaskPriorityHigh, AgentTaskPriorityNormal)
	assert.Greater(t, AgentTaskPriorityNormal, AgentTaskPriorityLow)
}
