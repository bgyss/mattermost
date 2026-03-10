// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// AgentTaskEvent types
const (
	AgentTaskEventTypeThinking   = "thinking"
	AgentTaskEventTypeToolCall   = "tool_call"
	AgentTaskEventTypeToolResult = "tool_result"
	AgentTaskEventTypeDelegation = "delegation"
	AgentTaskEventTypeCompletion = "completion"
	AgentTaskEventTypeError      = "error"
	AgentTaskEventTypeTokenChunk = "token_chunk"
)

// AgentTaskEvent is a single observability event emitted during task execution.
type AgentTaskEvent struct {
	Id        string                 `json:"id"`
	TaskId    string                 `json:"task_id"`
	AgentId   string                 `json:"agent_id"`
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreateAt  int64                  `json:"create_at"`
}

func (e *AgentTaskEvent) PreSave() {
	if e.Id == "" {
		e.Id = NewId()
	}
	e.CreateAt = GetMillis()
}
