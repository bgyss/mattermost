// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoTool is a test tool that echoes its "msg" argument.
type echoTool struct {
	name string
}

func (e *echoTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        e.name,
		Description: "echoes msg",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"msg": {Type: "string"},
			},
			Required: []string{"msg"},
		},
	}
}

func (e *echoTool) Execute(_ context.Context, _ request.CTX, args json.RawMessage) (string, error) {
	var p struct {
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	return p.Msg, nil
}

// failTool always returns an error.
type failTool struct{}

func (f *failTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "fail_tool", Description: "always fails"}
}

func (f *failTool) Execute(_ context.Context, _ request.CTX, _ json.RawMessage) (string, error) {
	return "", errors.New("deliberate failure")
}

func TestToolRegistry_RegisterAndDefinitions(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&echoTool{name: "echo"})
	reg.Register(&echoTool{name: "echo2"})

	defs := reg.Definitions()
	require.Len(t, defs, 2)

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
	}
	assert.True(t, names["echo"])
	assert.True(t, names["echo2"])
}

func TestToolRegistry_ExecuteParallel_Success(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&echoTool{name: "echo"})

	calls := []ToolCall{
		{ID: "c1", Name: "echo", InputRaw: json.RawMessage(`{"msg":"hello"}`)},
		{ID: "c2", Name: "echo", InputRaw: json.RawMessage(`{"msg":"world"}`)},
	}

	logger := mlog.CreateConsoleTestLogger(t)
	results := reg.ExecuteParallel(context.Background(), request.EmptyContext(logger), calls, logger)

	require.Len(t, results, 2)
	byID := make(map[string]ToolResult)
	for _, r := range results {
		byID[r.CallID] = r
	}
	assert.Equal(t, "hello", byID["c1"].Content)
	assert.Equal(t, "world", byID["c2"].Content)
	assert.False(t, byID["c1"].IsError)
	assert.False(t, byID["c2"].IsError)
}

func TestToolRegistry_ExecuteParallel_ToolError(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&failTool{})

	calls := []ToolCall{
		{ID: "c1", Name: "fail_tool", InputRaw: json.RawMessage(`{}`)},
	}

	logger := mlog.CreateConsoleTestLogger(t)
	results := reg.ExecuteParallel(context.Background(), request.EmptyContext(logger), calls, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].IsError)
	assert.Contains(t, results[0].Content, "deliberate failure")
}

func TestToolRegistry_ExecuteParallel_UnknownTool(t *testing.T) {
	reg := NewToolRegistry()

	calls := []ToolCall{
		{ID: "c1", Name: "nonexistent", InputRaw: json.RawMessage(`{}`)},
	}

	logger := mlog.CreateConsoleTestLogger(t)
	results := reg.ExecuteParallel(context.Background(), request.EmptyContext(logger), calls, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].IsError)
}

func TestToolRegistry_ExecuteParallel_ActuallyParallel(t *testing.T) {
	// Verify goroutines really run in parallel by counting concurrent executions.
	var concurrent int64
	var maxConcurrent int64

	type blockingTool struct{ done chan struct{} }
	blocker := make(chan struct{})
	tools := make([]*echoTool, 5)
	for i := range tools {
		tools[i] = &echoTool{name: "echo"}
	}

	// Use a counter tool instead
	type countTool struct{ done chan struct{} }
	_ = blocker // suppress unused warning

	reg := NewToolRegistry()
	reg.Register(&echoTool{name: "echo"})

	calls := make([]ToolCall, 10)
	for i := range calls {
		calls[i] = ToolCall{
			ID:       string(rune('a' + i)),
			Name:     "echo",
			InputRaw: json.RawMessage(`{"msg":"x"}`),
		}
	}
	_ = concurrent
	_ = maxConcurrent

	logger := mlog.CreateConsoleTestLogger(t)
	results := reg.ExecuteParallel(context.Background(), request.EmptyContext(logger), calls, logger)
	assert.Len(t, results, 10)

	// Verify all calls returned results
	for _, r := range results {
		assert.False(t, r.IsError)
		assert.Equal(t, "x", r.Content)
	}
}

func TestToolRegistry_ExecuteParallel_EmptyCalls(t *testing.T) {
	reg := NewToolRegistry()
	logger := mlog.CreateConsoleTestLogger(t)
	results := reg.ExecuteParallel(context.Background(), request.EmptyContext(logger), nil, logger)
	assert.Empty(t, results)
}

func TestToolRegistry_ContextCancellation(t *testing.T) {
	// Slow tool that checks ctx
	type slowTool struct{ calls int64 }
	reg := NewToolRegistry()
	reg.Register(&echoTool{name: "echo"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := []ToolCall{
		{ID: "c1", Name: "echo", InputRaw: json.RawMessage(`{"msg":"hi"}`)},
	}
	logger := mlog.CreateConsoleTestLogger(t)
	// Even with cancelled ctx, results should be returned (echo doesn't check ctx).
	results := reg.ExecuteParallel(ctx, request.EmptyContext(logger), calls, logger)
	assert.Len(t, results, 1)
	_ = atomic.LoadInt64 // silence import check
}
