// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// AgentAppIface is the subset of app.App the executor needs.
// Defined here to avoid an import cycle (agentruntime → channels/app would be circular).
type AgentAppIface interface {
	// Post operations
	CreatePost(rctx request.CTX, post *model.Post, channel *model.Channel, flags model.CreatePostFlags) (*model.Post, bool, *model.AppError)
	PatchPost(rctx request.CTX, postID string, patch *model.PostPatch, opts *model.UpdatePostOptions) (*model.Post, bool, *model.AppError)
	GetChannel(rctx request.CTX, channelID string) (*model.Channel, *model.AppError)
	GetChannelByName(rctx request.CTX, channelName, teamID string, includeDeleted bool) (*model.Channel, *model.AppError)
	GetPostThread(rctx request.CTX, postID string, opts model.GetPostsOptions, userID string) (*model.PostList, *model.AppError)
	GetPosts(rctx request.CTX, channelID string, offset int, limit int) (*model.PostList, *model.AppError)
	GetPublicChannelsForTeam(rctx request.CTX, teamID string, offset int, limit int) (model.ChannelList, *model.AppError)

	// User operations
	GetUser(userID string) (*model.User, *model.AppError)
	GetUserByUsername(username string) (*model.User, *model.AppError)

	// Search
	SearchPostsInTeam(teamID string, paramsList []*model.SearchParams) (*model.PostList, *model.AppError)

	// CreatePostMissingChannel creates a post without requiring the channel object (background use).
	CreatePostMissingChannel(rctx request.CTX, post *model.Post, triggerWebhooks bool, setOnline bool) (*model.Post, bool, *model.AppError)

	// WebSocket
	Publish(message *model.WebSocketEvent)
}

const (
	// streamFlushTokens is how many tokens to accumulate before flushing to the post.
	streamFlushTokens = 50
	// maxContextPosts is how many recent posts to include in conversation context.
	maxContextPosts = 20
	// maxToolRounds caps the agent loop to prevent runaway tool-calling.
	maxToolRounds = 10
)

// executor runs the core agentic loop for a single task.
type executor struct {
	store   store.Store
	app     AgentAppIface
	log     *mlog.Logger
	toolReg *ToolRegistry
	memMgr  *MemoryManager
	obs     *observabilityService
}

func newExecutor(st store.Store, app AgentAppIface, log *mlog.Logger, toolReg *ToolRegistry, memMgr *MemoryManager, obs *observabilityService) *executor {
	return &executor{store: st, app: app, log: log, toolReg: toolReg, memMgr: memMgr, obs: obs}
}

// Execute runs an agent task to completion. It is meant to be called in its own goroutine.
func (e *executor) Execute(task *model.AgentTask, def *model.AgentDefinition, llm LLMService) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Inject agent/task IDs so memory and other tools can scope their operations.
	ctx = context.WithValue(ctx, contextKeyAgentID, task.AgentId)
	ctx = context.WithValue(ctx, contextKeyTaskID, task.Id)

	logger := e.log.With(
		mlog.String("task_id", task.Id),
		mlog.String("agent_id", task.AgentId),
		mlog.String("agent_name", def.DisplayName),
	)

	rctx := request.EmptyContext(logger)

	// Mark running
	task.Status = model.AgentTaskStatusRunning
	task.UpdateAt = model.GetMillis()
	if _, err := e.store.Agent().UpdateAgentTask(task); err != nil {
		logger.Error("executor: failed to mark task running", mlog.Err(err))
		return
	}

	e.emitTaskEvent(task.Id, def.Id, model.AgentTaskEventTypeThinking,
		map[string]interface{}{"message": "Building context..."})

	// 1. Build initial conversation context from channel/thread history
	messages, ctxErr := e.buildContext(rctx, task, def)
	if ctxErr != nil {
		logger.Warn("executor: failed to build full context", mlog.Err(ctxErr))
	}

	// 2. Create the initial response post (shows "Thinking…" while streaming)
	responsePost, postErr := e.createResponsePost(rctx, task, def)
	if postErr != nil {
		e.failTask(task, fmt.Sprintf("failed to create response post: %s", postErr.Error()), startTime)
		return
	}
	task.ResponsePostId = responsePost.Id
	task.UpdateAt = model.GetMillis()
	if _, err := e.store.Agent().UpdateAgentTask(task); err != nil {
		logger.Warn("executor: failed to save response post id", mlog.Err(err))
	}

	e.publishTaskEvent(task, model.WebsocketEventAgentTaskSubmitted)

	// 3. Agentic loop: LLM → tool calls → results → LLM → … → final text
	toolDefs := e.toolReg.Definitions()
	var outputText string
	var totalInput, totalOutput int

	// Build system prompt once, prepending agent memory context.
	systemPrompt := def.SystemPrompt
	if e.memMgr != nil {
		systemPrompt += e.memMgr.BuildMemoryContext(task.AgentId)
	}

	for round := 0; round < maxToolRounds; round++ {
		llmReq := LLMRequest{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			ModelId:      def.ModelId,
			Parameters:   def.ModelParameters,
			Tools:        toolDefs,
		}

		stream, streamErr := llm.Stream(ctx, llmReq)
		if streamErr != nil {
			logger.Error("executor: LLM stream error", mlog.Err(streamErr))
			e.failTask(task, fmt.Sprintf("LLM stream failed: %s", streamErr.Error()), startTime)
			errMsg := "_Agent encountered an error and could not complete this request._"
			if _, _, patchErr := e.app.PatchPost(rctx, responsePost.Id, &model.PostPatch{Message: &errMsg}, &model.UpdatePostOptions{}); patchErr != nil {
				logger.Warn("executor: failed to update post with error", mlog.Err(patchErr))
			}
			return
		}

		// Consume the stream
		chunkText, toolCalls, inTok, outTok := e.consumeStream(ctx, rctx, stream, task, responsePost, logger)
		totalInput += inTok
		totalOutput += outTok

		if len(chunkText) > 0 {
			outputText = chunkText
		}

		// No tool calls requested — agent is done
		if len(toolCalls) == 0 {
			break
		}

		// --- Tool round ---
		logger.Debug("executor: tool round", mlog.Int("round", round), mlog.Int("tool_calls", len(toolCalls)))

		// Append the assistant's tool-request message to history
		messages = append(messages, LLMMessage{
			Role:      "assistant",
			ToolCalls: toolCalls,
		})

		// Execute all tool calls in parallel
		calls := make([]ToolCall, len(toolCalls))
		for i, tc := range toolCalls {
			calls[i] = ToolCall{
				ID:       tc.ID,
				Name:     tc.Name,
				InputRaw: json.RawMessage(tc.Input),
			}
		}

		results := e.toolReg.ExecuteParallel(ctx, rctx, calls, logger)

		// Emit observability events for each tool call/result
		for i, res := range results {
			e.emitTaskEvent(task.Id, def.Id, model.AgentTaskEventTypeToolCall, map[string]interface{}{
				"tool": calls[i].Name,
				"args": string(calls[i].InputRaw),
			})
			e.emitTaskEvent(task.Id, def.Id, model.AgentTaskEventTypeToolResult, map[string]interface{}{
				"tool":     res.Name,
				"result":   res.Content,
				"is_error": res.IsError,
			})

			// Broadcast thinking event so the frontend can show tool activity
			thinkEvt := model.NewWebSocketEvent(model.WebsocketEventAgentThinking, "", task.ChannelId, "", nil, "")
			thinkEvt.Add("task_id", task.Id)
			thinkEvt.Add("tool", res.Name)
			thinkEvt.Add("is_error", res.IsError)
			e.app.Publish(thinkEvt)

			// Append tool result message so the LLM sees it in the next round
			messages = append(messages, LLMMessage{
				Role:       "tool",
				Content:    res.Content,
				ToolCallId: res.CallID,
				ToolName:   res.Name,
			})
		}
		// Loop continues: LLM will incorporate tool results in next turn
	}

	// 4. Final patch with complete accumulated text
	if _, _, patchErr := e.app.PatchPost(rctx, responsePost.Id, &model.PostPatch{Message: &outputText}, &model.UpdatePostOptions{}); patchErr != nil {
		logger.Warn("executor: final patch failed", mlog.Err(patchErr))
	}

	// 5. Mark complete
	latencyMs := time.Since(startTime).Milliseconds()
	task.Status = model.AgentTaskStatusComplete
	task.Output = model.AgentTaskOutput{Text: outputText}
	task.TokensUsed = totalInput + totalOutput
	task.LatencyMs = latencyMs
	task.CompleteAt = model.GetMillis()
	task.UpdateAt = task.CompleteAt
	if _, err := e.store.Agent().UpdateAgentTask(task); err != nil {
		logger.Warn("executor: failed to mark task complete", mlog.Err(err))
	}

	e.emitTaskEvent(task.Id, def.Id, model.AgentTaskEventTypeCompletion, map[string]interface{}{
		"tokens_input":  totalInput,
		"tokens_output": totalOutput,
		"latency_ms":    latencyMs,
	})
	e.publishTaskEvent(task, model.WebsocketEventAgentTaskComplete)

	if e.obs != nil {
		e.obs.RecordComplete(task.AgentId, int64(totalInput+totalOutput), latencyMs)
	}

	logger.Info("executor: task complete",
		mlog.Int("tokens_total", totalInput+totalOutput),
		mlog.Int("latency_ms", latencyMs),
	)
}

// buildContext assembles the LLM conversation history from recent channel posts.
func (e *executor) buildContext(rctx request.CTX, task *model.AgentTask, def *model.AgentDefinition) ([]LLMMessage, error) {
	var messages []LLMMessage
	var posts []*model.Post

	if task.ThreadRootPostId != "" {
		thread, err := e.app.GetPostThread(rctx, task.ThreadRootPostId, model.GetPostsOptions{}, def.BotUserId)
		if err != nil {
			return nil, fmt.Errorf("get thread: %w", err)
		}
		for _, p := range thread.ToSlice() {
			posts = append(posts, p)
		}
	} else {
		postList, err := e.app.GetPosts(rctx, task.ChannelId, 0, maxContextPosts)
		if err != nil {
			return nil, fmt.Errorf("get posts: %w", err)
		}
		posts = postList.ToSlice()
	}

	for _, p := range posts {
		if p.Type != "" || p.Message == "" {
			continue
		}
		role := "user"
		if p.UserId == def.BotUserId {
			role = "assistant"
		}
		messages = append(messages, LLMMessage{Role: role, Content: p.Message})
	}

	// Ensure the task input is the last user message if not already present
	if task.Input.Text != "" {
		needsAppend := true
		if len(messages) > 0 {
			last := messages[len(messages)-1]
			if last.Role == "user" && strings.TrimSpace(last.Content) == strings.TrimSpace(task.Input.Text) {
				needsAppend = false
			}
		}
		if needsAppend {
			messages = append(messages, LLMMessage{Role: "user", Content: task.Input.Text})
		}
	}

	return messages, nil
}

// createResponsePost creates an initial placeholder post for the agent's response.
func (e *executor) createResponsePost(rctx request.CTX, task *model.AgentTask, def *model.AgentDefinition) (*model.Post, error) {
	channel, chErr := e.app.GetChannel(rctx, task.ChannelId)
	if chErr != nil {
		return nil, fmt.Errorf("get channel: %w", chErr)
	}

	post := &model.Post{
		UserId:    def.BotUserId,
		ChannelId: task.ChannelId,
		Message:   "_Thinking…_",
		Props: model.StringInterface{
			"from_agent":         true,
			"agent_task_id":      task.Id,
			"agent_root_task_id": task.RootTaskId,
			"ai_generated_by":    def.Id,
		},
	}
	if task.ThreadRootPostId != "" {
		post.RootId = task.ThreadRootPostId
	} else if task.RequestPostId != "" {
		post.RootId = task.RequestPostId
	}

	created, _, appErr := e.app.CreatePost(rctx, post, channel, model.CreatePostFlags{})
	if appErr != nil {
		return nil, fmt.Errorf("create post: %w", appErr)
	}
	return created, nil
}

// consumeStream reads chunks from the LLM stream. It buffers text, periodically
// flushing to the response post and emitting WebSocket events.
// Returns (accumulatedText, toolCalls, inputTokens, outputTokens).
func (e *executor) consumeStream(
	ctx context.Context,
	rctx request.CTX,
	stream <-chan LLMChunk,
	task *model.AgentTask,
	post *model.Post,
	logger *mlog.Logger,
) (string, []LLMToolCall, int, int) {
	var buf strings.Builder
	tokensSinceFlush := 0
	var inputTokens, outputTokens int

	for chunk := range stream {
		select {
		case <-ctx.Done():
			return buf.String(), nil, inputTokens, outputTokens
		default:
		}

		if chunk.Error != nil {
			logger.Warn("executor: stream error", mlog.Err(chunk.Error))
			break
		}

		if chunk.Done {
			inputTokens = chunk.InputTokens
			outputTokens = chunk.OutputTokens
			if len(chunk.ToolCalls) > 0 {
				return buf.String(), chunk.ToolCalls, inputTokens, outputTokens
			}
			break
		}

		buf.WriteString(chunk.Text)
		tokensSinceFlush++

		if tokensSinceFlush >= streamFlushTokens {
			current := buf.String()
			streamEvt := model.NewWebSocketEvent(model.WebsocketEventAgentTokenStream, "", task.ChannelId, "", nil, "")
			streamEvt.Add("task_id", task.Id)
			streamEvt.Add("post_id", post.Id)
			streamEvt.Add("text", current)
			e.app.Publish(streamEvt)

			if _, _, err := e.app.PatchPost(rctx, post.Id, &model.PostPatch{Message: &current}, &model.UpdatePostOptions{}); err != nil {
				logger.Warn("executor: patch post during stream", mlog.Err(err))
			}
			tokensSinceFlush = 0
		}
	}

	return buf.String(), nil, inputTokens, outputTokens
}

// failTask marks a task as failed in the store and emits a WS event.
func (e *executor) failTask(task *model.AgentTask, errMsg string, startTime time.Time) {
	task.Status = model.AgentTaskStatusFailed
	task.ErrorMsg = errMsg
	task.LatencyMs = time.Since(startTime).Milliseconds()
	task.CompleteAt = model.GetMillis()
	task.UpdateAt = task.CompleteAt
	if _, err := e.store.Agent().UpdateAgentTask(task); err != nil {
		e.log.Warn("executor: failed to persist task failure", mlog.Err(err), mlog.String("task_id", task.Id))
	}
	e.publishTaskEvent(task, model.WebsocketEventAgentTaskFailed)
	if e.obs != nil {
		e.obs.RecordFail(task.AgentId)
	}
}

// emitTaskEvent persists an AgentTaskEvent row for observability.
func (e *executor) emitTaskEvent(taskId, agentId, eventType string, payload map[string]interface{}) {
	evt := &model.AgentTaskEvent{
		TaskId:    taskId,
		AgentId:   agentId,
		EventType: eventType,
		Payload:   payload,
	}
	if _, err := e.store.Agent().SaveAgentTaskEvent(evt); err != nil {
		e.log.Warn("executor: failed to save task event",
			mlog.Err(err), mlog.String("event_type", eventType))
	}
}

// publishTaskEvent sends a WebSocket event for a task status change.
func (e *executor) publishTaskEvent(task *model.AgentTask, eventType model.WebsocketEventType) {
	evt := model.NewWebSocketEvent(eventType, "", task.ChannelId, "", nil, "")
	evt.Add("task_id", task.Id)
	evt.Add("agent_id", task.AgentId)
	evt.Add("status", task.Status)
	e.app.Publish(evt)
}
