// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

// contextKey is an unexported type for context keys scoped to this package.
type contextKey string

const (
	// contextKeyAgentID stores the executing agent's ID in the task context.
	// Memory tools read this to scope their store operations.
	contextKeyAgentID contextKey = "agent_id"
	// contextKeyTaskID stores the current task's ID in the task context.
	contextKeyTaskID contextKey = "task_id"
	// contextKeyUserId stores the requesting user's ID if known.
	contextKeyUserID contextKey = "user_id"
)
