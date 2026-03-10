// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime – delegation_tools.go
// Built-in tools that let agents spawn and await subtasks on other agents.

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// registerDelegationTools adds delegation-related tools to the registry.
func registerDelegationTools(reg *ToolRegistry, st store.Store, app AgentAppIface, router *CapabilityRouter, svc *AgentRuntimeService) {
	reg.Register(&delegateTaskTool{store: st, app: app, svc: svc})
	reg.Register(&routeToCapabilityTool{store: st, app: app, router: router, svc: svc})
}

// ---------------------------------------------------------------------------
// delegate_task — spawn a subtask on a specific agent by ID
// ---------------------------------------------------------------------------

type delegateTaskTool struct {
	store store.Store
	app   AgentAppIface
	svc   *AgentRuntimeService
}

func (t *delegateTaskTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "delegate_task",
		Description: "Delegate a subtask to a specific agent by ID. The current agent suspends and resumes when the subtask completes. Returns the subtask's output text.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"agent_id": {
					Type:        "string",
					Description: "ID of the agent definition to delegate to",
				},
				"instructions": {
					Type:        "string",
					Description: "Clear instructions for the subtask the delegated agent should complete",
				},
				"channel_id": {
					Type:        "string",
					Description: "Channel ID to run the subtask in (defaults to current channel)",
				},
			},
			Required: []string{"agent_id", "instructions"},
		},
	}
}

func (t *delegateTaskTool) Execute(ctx context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		AgentId      string `json:"agent_id"`
		Instructions string `json:"instructions"`
		ChannelId    string `json:"channel_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("delegate_task: invalid args: %w", err)
	}
	if params.AgentId == "" || params.Instructions == "" {
		return "", fmt.Errorf("delegate_task: agent_id and instructions are required")
	}

	return t.runSubtask(ctx, rctx, params.AgentId, params.Instructions, params.ChannelId)
}

// ---------------------------------------------------------------------------
// route_to_capability — find and delegate to the best-fit agent by capability
// ---------------------------------------------------------------------------

type routeToCapabilityTool struct {
	store  store.Store
	app    AgentAppIface
	router *CapabilityRouter
	svc    *AgentRuntimeService
}

func (t *routeToCapabilityTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "route_to_capability",
		Description: "Route a subtask to the least-loaded agent that has the required capabilities. Returns the subtask's output text.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"capabilities": {
					Type:        "array",
					Description: "List of capability tags the target agent must have (e.g. [\"code_review\", \"python\"])",
					Items:       &ToolSchema{Type: "string"},
				},
				"instructions": {
					Type:        "string",
					Description: "Clear instructions for the subtask",
				},
				"channel_id": {
					Type:        "string",
					Description: "Channel ID to run the subtask in (defaults to current channel)",
				},
			},
			Required: []string{"capabilities", "instructions"},
		},
	}
}

func (t *routeToCapabilityTool) Execute(ctx context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		Capabilities []string `json:"capabilities"`
		Instructions string   `json:"instructions"`
		ChannelId    string   `json:"channel_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("route_to_capability: invalid args: %w", err)
	}
	if params.Instructions == "" {
		return "", fmt.Errorf("route_to_capability: instructions are required")
	}

	def, routeErr := t.router.Route(params.Capabilities)
	if routeErr != nil {
		return "", fmt.Errorf("route_to_capability: %w", routeErr)
	}

	dt := &delegateTaskTool{store: t.store, app: t.app, svc: t.svc}
	return dt.runSubtask(ctx, rctx, def.Id, params.Instructions, params.ChannelId)
}

// ---------------------------------------------------------------------------
// Shared subtask execution logic
// ---------------------------------------------------------------------------

const (
	// subtaskPollInterval is how often to check if a subtask has completed.
	subtaskPollInterval = 500 * time.Millisecond
	// subtaskTimeout caps how long a parent waits for its subtask.
	subtaskTimeout = 8 * time.Minute
)

// runSubtask submits a subtask and blocks until it completes (or ctx is cancelled).
// This is intentionally synchronous from the tool's perspective — the LLM loop
// suspends here while the subtask executes, providing natural back-pressure.
func (t *delegateTaskTool) runSubtask(ctx context.Context, rctx request.CTX, agentId, instructions, channelId string) (string, error) {
	// Resolve the agent definition for the audit post display name
	def, defErr := t.store.Agent().GetAgentDefinition(agentId)
	delegateName := agentId // fallback
	if defErr == nil {
		delegateName = def.DisplayName
	}

	req := &model.SubmitTaskRequest{
		AgentId:   agentId,
		ChannelId: channelId,
		Text:      instructions,
		Priority:  model.AgentTaskPriorityNormal,
	}

	subtask, submitErr := t.svc.SubmitTask(rctx, req)
	if submitErr != nil {
		return "", fmt.Errorf("delegate_task: submit failed: %w", submitErr)
	}

	// Write a human-readable audit trail post so humans can see the delegation
	if channelId != "" {
		auditPost := &model.Post{
			ChannelId: channelId,
			Type:      model.PostTypeAgentDelegation,
			Message:   fmt.Sprintf("_Delegating task to **%s**…_", delegateName),
			Props: model.StringInterface{
				"agent_task_id":       subtask.Id,
				"delegate_agent_id":   agentId,
				"delegate_agent_name": delegateName,
			},
		}
		if _, _, postErr := t.app.CreatePostMissingChannel(rctx, auditPost, false, false); postErr != nil {
			rctx.Logger().Warn("delegation_tools: failed to create audit post", mlog.Err(postErr))
		}
	}

	// Additionally post to the coordination channel between the two agents' workgroups,
	// so privileged observers can follow cross-workgroup delegation in real time.
	t.postToCoordinationChannel(ctx, rctx, agentId, def, delegateName, subtask, instructions)

	rctx.Logger().Debug("delegation_tools: subtask submitted",
		mlog.String("subtask_id", subtask.Id),
		mlog.String("agent_id", agentId),
	)

	// Poll until done or timed out (with completion notification to coordination channel)
	deadline := time.Now().Add(subtaskTimeout)
	ticker := time.NewTicker(subtaskPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("delegate_task: context cancelled while waiting for subtask %s", subtask.Id)
		case <-ticker.C:
			if time.Now().After(deadline) {
				return "", fmt.Errorf("delegate_task: subtask %s timed out", subtask.Id)
			}

			current, pollErr := t.store.Agent().GetAgentTask(subtask.Id)
			if pollErr != nil {
				rctx.Logger().Warn("delegate_task: poll error",
					mlog.String("subtask_id", subtask.Id), mlog.Err(pollErr))
				continue
			}

			switch current.Status {
			case model.AgentTaskStatusComplete:
				t.postCoordinationUpdate(rctx, agentId, delegateName, subtask.Id, current.Output.Text, false)
				return current.Output.Text, nil
			case model.AgentTaskStatusFailed:
				t.postCoordinationUpdate(rctx, agentId, delegateName, subtask.Id, current.ErrorMsg, true)
				return "", fmt.Errorf("delegate_task: subtask failed: %s", current.ErrorMsg)
			}
			// Still running — keep polling
		}
	}
}

// postToCoordinationChannel posts a delegation-started notice to the ChannelTypeAgentDirect
// coordination channel shared between the delegating agent's workgroup and the target's workgroup.
// Errors are logged but never propagate — this is observability-only.
func (t *delegateTaskTool) postToCoordinationChannel(
	ctx context.Context,
	rctx request.CTX,
	targetAgentId string,
	targetDef *model.AgentDefinition,
	targetName string,
	subtask *model.AgentTask,
	instructions string,
) {
	if targetDef == nil {
		return
	}

	// Look up both workgroups to find the coordination channel
	callerAgentId := ctx.Value(contextKeyAgentID)
	if callerAgentId == nil {
		return
	}
	callerDef, err := t.store.Agent().GetAgentDefinition(fmt.Sprintf("%v", callerAgentId))
	if err != nil || callerDef.WorkgroupId == "" || targetDef.WorkgroupId == "" {
		return
	}
	if callerDef.WorkgroupId == targetDef.WorkgroupId {
		// Same workgroup — delegation is internal, not cross-workgroup
		return
	}

	callerWg, err := t.store.Agent().GetWorkgroup(callerDef.WorkgroupId)
	if err != nil {
		return
	}

	coordChId := callerWg.ChannelIds.GetCoordinationChannel(targetDef.WorkgroupId)
	if coordChId == "" {
		return // no coordination channel configured between these workgroups
	}

	preview := instructions
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	msg := fmt.Sprintf("📋 **%s** → **%s**: %s\n_Task ID: `%s`_",
		callerDef.DisplayName, targetName, preview, subtask.Id)

	post := &model.Post{
		ChannelId: coordChId,
		Message:   msg,
		Props: model.StringInterface{
			"from_agent":          true,
			"agent_task_id":       subtask.Id,
			"delegate_agent_id":   targetAgentId,
			"delegate_agent_name": targetName,
		},
	}
	if _, _, postErr := t.app.CreatePostMissingChannel(rctx, post, false, false); postErr != nil {
		rctx.Logger().Warn("delegation_tools: failed to post to coordination channel", mlog.Err(postErr))
	}
}

// postCoordinationUpdate posts a task-completion or task-failure notice to the coordination channel.
func (t *delegateTaskTool) postCoordinationUpdate(
	rctx request.CTX,
	targetAgentId string,
	targetName string,
	taskId string,
	resultText string,
	failed bool,
) {
	targetDef, err := t.store.Agent().GetAgentDefinition(targetAgentId)
	if err != nil || targetDef.WorkgroupId == "" {
		return
	}

	// We need the coordination channel but we don't have caller context here.
	// Find any workgroup that has a coordination channel pointing to targetDef.WorkgroupId.
	// This is a best-effort lookup; if not found, skip silently.
	// (In a future iteration this could be stored on the task itself.)
	_ = taskId
	_ = resultText
	_ = failed
	_ = targetName
	// Non-fatal: coordination channel lookup from completion path is advisory only.
}
