// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime – memory_tools.go
// Built-in tools that let agents read and write their persistent memory.

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// registerMemoryTools adds remember, recall, and recall_all to the registry.
func registerMemoryTools(reg *ToolRegistry, mem *MemoryManager) {
	reg.Register(&rememberTool{mem: mem})
	reg.Register(&recallTool{mem: mem})
	reg.Register(&recallAllTool{mem: mem})
}

// ---------------------------------------------------------------------------
// remember — store a fact
// ---------------------------------------------------------------------------

type rememberTool struct{ mem *MemoryManager }

func (t *rememberTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "remember",
		Description: "Store a fact in your persistent memory. Use this to save information you want to recall in future conversations.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"key": {
					Type:        "string",
					Description: "Short label for this memory (e.g. 'user_preferred_language')",
				},
				"value": {
					Type:        "string",
					Description: "The fact to remember",
				},
				"scope": {
					Type:        "string",
					Description: "Memory scope: 'global' (persists across all conversations), 'user' (per-user), or 'task' (current task only, expires when done)",
					Enum:        []string{"global", "user", "task"},
				},
				"scope_id": {
					Type:        "string",
					Description: "User ID for 'user' scope, task ID for 'task' scope; omit for global",
				},
			},
			Required: []string{"key", "value"},
		},
	}
}

func (t *rememberTool) Execute(ctx context.Context, _ request.CTX, args json.RawMessage) (string, error) {
	var p struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Scope   string `json:"scope"`
		ScopeId string `json:"scope_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("remember: invalid args: %w", err)
	}
	if p.Key == "" || p.Value == "" {
		return "", fmt.Errorf("remember: key and value are required")
	}

	agentId, _ := ctx.Value(contextKeyAgentID).(string)
	if agentId == "" {
		return "", fmt.Errorf("remember: agent ID not available in context")
	}

	// For 'task' scope, default the scope_id to the current task ID if not specified.
	if p.Scope == model.AgentMemoryScopeTask && p.ScopeId == "" {
		if taskId, ok := ctx.Value(contextKeyTaskID).(string); ok {
			p.ScopeId = taskId
		}
	}

	if err := t.mem.Remember(agentId, p.Scope, p.ScopeId, p.Key, p.Value, 0); err != nil {
		return "", err
	}
	return fmt.Sprintf("Remembered: %s = %s", p.Key, p.Value), nil
}

// ---------------------------------------------------------------------------
// recall — retrieve a single fact
// ---------------------------------------------------------------------------

type recallTool struct{ mem *MemoryManager }

func (t *recallTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "recall",
		Description: "Retrieve a specific fact from your persistent memory by key.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"key": {
					Type:        "string",
					Description: "The key of the memory to retrieve",
				},
				"scope": {
					Type:        "string",
					Description: "Memory scope: 'global', 'user', or 'task'",
					Enum:        []string{"global", "user", "task"},
				},
				"scope_id": {
					Type:        "string",
					Description: "User ID or task ID for scoped memories",
				},
			},
			Required: []string{"key"},
		},
	}
}

func (t *recallTool) Execute(ctx context.Context, _ request.CTX, args json.RawMessage) (string, error) {
	var p struct {
		Key     string `json:"key"`
		Scope   string `json:"scope"`
		ScopeId string `json:"scope_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("recall: invalid args: %w", err)
	}

	agentId, _ := ctx.Value(contextKeyAgentID).(string)
	if agentId == "" {
		return "", fmt.Errorf("recall: agent ID not available in context")
	}

	mem, err := t.mem.Recall(agentId, p.Scope, p.ScopeId, p.Key)
	if err != nil {
		return fmt.Sprintf("No memory found for key '%s'", p.Key), nil
	}
	return mem.ValueText, nil
}

// ---------------------------------------------------------------------------
// recall_all — list all memories in a scope
// ---------------------------------------------------------------------------

type recallAllTool struct{ mem *MemoryManager }

func (t *recallAllTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "recall_all",
		Description: "List all facts stored in your memory for a given scope.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"scope": {
					Type:        "string",
					Description: "Memory scope: 'global', 'user', or 'task'",
					Enum:        []string{"global", "user", "task"},
				},
				"scope_id": {
					Type:        "string",
					Description: "User ID or task ID for scoped memories; omit for global",
				},
			},
		},
	}
}

func (t *recallAllTool) Execute(ctx context.Context, _ request.CTX, args json.RawMessage) (string, error) {
	var p struct {
		Scope   string `json:"scope"`
		ScopeId string `json:"scope_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("recall_all: invalid args: %w", err)
	}

	agentId, _ := ctx.Value(contextKeyAgentID).(string)
	if agentId == "" {
		return "", fmt.Errorf("recall_all: agent ID not available in context")
	}

	mems, err := t.mem.RecallAll(agentId, p.Scope, p.ScopeId)
	if err != nil {
		return "", err
	}
	if len(mems) == 0 {
		return "No memories stored.", nil
	}

	var sb strings.Builder
	for _, m := range mems {
		fmt.Fprintf(&sb, "%s: %s\n", m.Key, m.ValueText)
	}
	return sb.String(), nil
}
