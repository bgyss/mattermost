// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// ToolSchema describes a single parameter for a tool (JSON Schema subset).
type ToolSchema struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]*ToolSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Items       *ToolSchema            `json:"items,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
}

// ToolDefinition is the schema sent to the LLM to describe a callable tool.
type ToolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  ToolSchema `json:"parameters"`
}

// ToolCall is a single tool invocation requested by the LLM.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	InputRaw json.RawMessage `json:"input"` // parsed from LLM response
}

// ToolResult is the result of executing a ToolCall.
type ToolResult struct {
	CallID  string
	Name    string
	Content string
	IsError bool
}

// AgentTool is the interface every tool must implement.
type AgentTool interface {
	// Definition returns the JSON schema sent to the LLM.
	Definition() ToolDefinition
	// Execute runs the tool. ctx carries the task context, rctx for app calls.
	// args is the raw JSON the LLM provided for this call.
	Execute(ctx context.Context, rctx request.CTX, args json.RawMessage) (string, error)
}

// ToolRegistry holds all registered tools indexed by name.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]AgentTool
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]AgentTool)}
}

// Register adds a tool; panics if a tool with the same name is already registered.
func (r *ToolRegistry) Register(t AgentTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Definition().Name
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tool already registered: %q", name))
	}
	r.tools[name] = t
}

// Get returns a tool by name, or nil.
func (r *ToolRegistry) Get(name string) AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Definitions returns all tool definitions (sent to the LLM in each request).
func (r *ToolRegistry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// ExecuteParallel runs all pending tool calls in parallel, respecting ctx cancellation.
// Returns results in the same order as calls.
func (r *ToolRegistry) ExecuteParallel(
	ctx context.Context,
	rctx request.CTX,
	calls []ToolCall,
	log *mlog.Logger,
) []ToolResult {
	results := make([]ToolResult, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc ToolCall) {
			defer wg.Done()
			tool := r.Get(tc.Name)
			if tool == nil {
				results[idx] = ToolResult{
					CallID:  tc.ID,
					Name:    tc.Name,
					Content: fmt.Sprintf("unknown tool: %q", tc.Name),
					IsError: true,
				}
				return
			}

			content, err := tool.Execute(ctx, rctx, tc.InputRaw)
			if err != nil {
				log.Warn("tool execution error",
					mlog.String("tool", tc.Name), mlog.Err(err))
				results[idx] = ToolResult{
					CallID:  tc.ID,
					Name:    tc.Name,
					Content: fmt.Sprintf("error: %s", err.Error()),
					IsError: true,
				}
				return
			}
			results[idx] = ToolResult{CallID: tc.ID, Name: tc.Name, Content: content}
		}(i, call)
	}

	wg.Wait()
	return results
}
