// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// AgentMemory scopes
const (
	AgentMemoryScopeGlobal = "global"
	AgentMemoryScopeTask   = "task"
	AgentMemoryScopeUser   = "user"
)

// AgentMemory stores a key/value memory entry for an agent.
// ValueVec (pgvector embedding) is stored separately in the DB but not carried in this struct
// to avoid pulling in the pgvector driver into the public model package.
type AgentMemory struct {
	Id        string `json:"id"`
	AgentId   string `json:"agent_id"`
	Scope     string `json:"scope"`    // global|task|user
	ScopeId   string `json:"scope_id"` // TaskId or UserId depending on Scope; "" for global
	Key       string `json:"key"`
	ValueText string `json:"value_text"`
	ExpireAt  int64  `json:"expire_at"` // 0 = no expiry
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
}

func (m *AgentMemory) PreSave() {
	if m.Id == "" {
		m.Id = NewId()
	}
	now := GetMillis()
	m.CreateAt = now
	m.UpdateAt = now
}

func (m *AgentMemory) PreUpdate() {
	m.UpdateAt = GetMillis()
}
