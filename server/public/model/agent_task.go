// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// AgentTask statuses
const (
	AgentTaskStatusPending         = "pending"
	AgentTaskStatusClaimed         = "claimed"
	AgentTaskStatusRunning         = "running"
	AgentTaskStatusAwaitingSubtask = "awaiting_subtask"
	AgentTaskStatusComplete        = "complete"
	AgentTaskStatusFailed          = "failed"
)

// AgentTask priorities
const (
	AgentTaskPriorityLow    = 10
	AgentTaskPriorityNormal = 50
	AgentTaskPriorityHigh   = 80
	AgentTaskPriorityUrgent = 100
)

// AgentTaskInput is the structured input to an agent task.
type AgentTaskInput struct {
	Text      string                 `json:"text"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AgentTaskOutput is the structured output from a completed agent task.
type AgentTaskOutput struct {
	Text      string                 `json:"text"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AgentTask is the atomic unit of work dispatched to an agent.
type AgentTask struct {
	Id                string          `json:"id"`
	AgentId           string          `json:"agent_id"`
	ParentTaskId      string          `json:"parent_task_id"`    // empty for root tasks
	RootTaskId        string          `json:"root_task_id"`      // same as Id for root tasks
	ChannelId         string          `json:"channel_id"`
	ThreadRootPostId  string          `json:"thread_root_post_id"`
	RequestPostId     string          `json:"request_post_id"`
	ResponsePostId    string          `json:"response_post_id"`
	Input             AgentTaskInput  `json:"input"`
	Output            AgentTaskOutput `json:"output"`
	Status            string          `json:"status"`
	Priority          int             `json:"priority"`
	DelegationChain   []string        `json:"delegation_chain"` // ordered list of agentIds from root
	TokensUsed        int             `json:"tokens_used"`
	LatencyMs         int64           `json:"latency_ms"`
	ErrorMsg          string          `json:"error_msg,omitempty"`
	CreateAt          int64           `json:"create_at"`
	UpdateAt          int64           `json:"update_at"`
	ClaimedAt         int64           `json:"claimed_at"`
	CompleteAt        int64           `json:"complete_at"`
	ClaimedByServer   string          `json:"claimed_by_server"` // server ID for cluster-safe claiming
}

func (t *AgentTask) IsValid() *AppError {
	if t.Id == "" {
		return NewAppError("AgentTask.IsValid", "model.agent_task.is_valid.id.app_error", nil, "", 400)
	}
	if t.AgentId == "" {
		return NewAppError("AgentTask.IsValid", "model.agent_task.is_valid.agent_id.app_error", nil, "", 400)
	}
	return nil
}

func (t *AgentTask) PreSave() {
	if t.Id == "" {
		t.Id = NewId()
	}
	if t.RootTaskId == "" {
		t.RootTaskId = t.Id
	}
	if t.Priority == 0 {
		t.Priority = AgentTaskPriorityNormal
	}
	if t.DelegationChain == nil {
		t.DelegationChain = []string{}
	}
	now := GetMillis()
	t.CreateAt = now
	t.UpdateAt = now
	t.Status = AgentTaskStatusPending
}

// SubmitTaskRequest is the request body for POST /api/v4/agents/tasks.
type SubmitTaskRequest struct {
	AgentId          string `json:"agent_id"`
	ChannelId        string `json:"channel_id"`
	RequestPostId    string `json:"request_post_id"`
	ThreadRootPostId string `json:"thread_root_post_id,omitempty"`
	Text             string `json:"text"`
	Priority         int    `json:"priority,omitempty"`
}
