// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentMemory_PreSave(t *testing.T) {
	t.Run("generates ID and timestamps", func(t *testing.T) {
		mem := &AgentMemory{
			AgentId:   NewId(),
			Scope:     AgentMemoryScopeGlobal,
			Key:       "test_key",
			ValueText: "test_value",
		}
		mem.PreSave()

		require.NotEmpty(t, mem.Id)
		assert.Greater(t, mem.CreateAt, int64(0))
		assert.Greater(t, mem.UpdateAt, int64(0))
		assert.Equal(t, mem.CreateAt, mem.UpdateAt)
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		id := NewId()
		mem := &AgentMemory{Id: id}
		mem.PreSave()
		assert.Equal(t, id, mem.Id)
	})

	t.Run("zero ExpireAt remains zero", func(t *testing.T) {
		mem := &AgentMemory{AgentId: NewId(), Key: "k"}
		mem.PreSave()
		assert.Equal(t, int64(0), mem.ExpireAt)
	})
}

func TestAgentMemory_PreUpdate(t *testing.T) {
	mem := &AgentMemory{UpdateAt: 1}
	mem.PreUpdate()
	assert.Greater(t, mem.UpdateAt, int64(1))
}

func TestAgentMemoryScope_Constants(t *testing.T) {
	assert.Equal(t, "global", AgentMemoryScopeGlobal)
	assert.Equal(t, "task", AgentMemoryScopeTask)
	assert.Equal(t, "user", AgentMemoryScopeUser)
}
