// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// LLMMessage is a single message in the conversation history.
type LLMMessage struct {
	Role       string        `json:"role"` // "user" | "assistant" | "system" | "tool"
	Content    string        `json:"content"`
	ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`   // populated for role=assistant when LLM requests tools
	ToolCallId string        `json:"tool_call_id,omitempty"` // populated for role=tool (result messages)
	ToolName   string        `json:"tool_name,omitempty"`    // populated for role=tool
}

// LLMToolCall represents a tool invocation requested by the LLM.
type LLMToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // raw JSON string of arguments
}

// LLMRequest is the input to an LLM provider.
type LLMRequest struct {
	SystemPrompt string
	Messages     []LLMMessage
	ModelId      string
	Parameters   model.ModelParameters
	// Tools are sent to the LLM so it can decide to call them.
	// Empty means no tool use.
	Tools []ToolDefinition
}

// LLMChunk is a single streamed token/chunk from the LLM.
type LLMChunk struct {
	Text  string
	Done  bool
	Error error
	// Total tokens used (populated in final Done chunk)
	InputTokens  int
	OutputTokens int
	// ToolCalls is populated (non-nil) when the LLM requests tool invocations.
	// A non-nil ToolCalls slice means the LLM did not produce text — it wants tools run.
	ToolCalls []LLMToolCall
}

// LLMService is the abstraction over any LLM provider.
type LLMService interface {
	// Stream sends a request to the LLM and returns a channel of chunks.
	// The caller must drain the channel until it is closed.
	// The last chunk has Done=true and may carry token usage totals or ToolCalls.
	Stream(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error)

	// ProviderName returns a human-readable name (e.g., "anthropic", "openai").
	ProviderName() string
}

// LLMServiceConfig holds the configuration needed to construct an LLM provider client.
type LLMServiceConfig struct {
	// Provider selects the LLM backend: "openclaw" (default), "anthropic", "openai".
	Provider string
	// APIKey / token for the provider.
	// For OpenClaw: the gateway bearer token (OPENCLAW_GATEWAY_TOKEN).
	APIKey string
	// BaseURL overrides the provider's default endpoint.
	// For OpenClaw: defaults to http://127.0.0.1:18789.
	BaseURL string
	// ExternalAgentId is forwarded as x-openclaw-agent-id for OpenClaw routing,
	// or prepended as "openclaw:<id>" in the model field.
	// For other providers it is ignored.
	ExternalAgentId string
}

// NewLLMService constructs the appropriate LLM client for the given config.
// Defaults to OpenClaw when Provider is empty.
func NewLLMService(cfg LLMServiceConfig) (LLMService, error) {
	switch strings.ToLower(cfg.Provider) {
	case "openclaw", "":
		return newOpenClawClient(cfg), nil
	case "anthropic":
		return newAnthropicClient(cfg), nil
	case "openai":
		return newOpenAIClient(cfg), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", cfg.Provider)
	}
}

// ---------------------------------------------------------------------------
// OpenClaw adapter (default)
// OpenClaw exposes an OpenAI-compatible /v1/chat/completions endpoint on the
// local gateway with agent selection via the x-openclaw-agent-id header.
// ---------------------------------------------------------------------------

const (
	defaultOpenClawBaseURL = "http://127.0.0.1:18789"
	defaultOpenClawAgentId = "main"
)

type openClawClient struct {
	cfg        LLMServiceConfig
	httpClient *http.Client
}

func newOpenClawClient(cfg LLMServiceConfig) *openClawClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenClawBaseURL
	}
	if cfg.ExternalAgentId == "" {
		cfg.ExternalAgentId = defaultOpenClawAgentId
	}
	return &openClawClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *openClawClient) ProviderName() string { return "openclaw" }

func (c *openClawClient) Stream(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	modelId := req.ModelId
	if modelId == "" {
		modelId = "openclaw"
	}

	msgs := make([]openAIMsg, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, openAIMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		if m.Role == "tool" {
			msgs = append(msgs, openAIMsg{
				Role:       "tool",
				Content:    m.Content,
				ToolCallId: m.ToolCallId,
			})
			continue
		}
		if len(m.ToolCalls) > 0 {
			oaiToolCalls := make([]openAIToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				oaiToolCalls = append(oaiToolCalls, openAIToolCall{
					Id:   tc.ID,
					Type: "function",
					Function: openAIToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
			msgs = append(msgs, openAIMsg{
				Role:      m.Role,
				Content:   m.Content,
				ToolCalls: oaiToolCalls,
			})
			continue
		}
		msgs = append(msgs, openAIMsg{Role: m.Role, Content: m.Content})
	}

	body := openAIChatRequest{
		Model:       modelId,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   req.Parameters.MaxTokens,
		Temperature: req.Parameters.Temperature,
		TopP:        req.Parameters.TopP,
		StreamOpts:  &openAIStreamOpts{IncludeUsage: true},
	}

	if len(req.Tools) > 0 {
		oaiTools := make([]openAITool, 0, len(req.Tools))
		for _, td := range req.Tools {
			oaiTools = append(oaiTools, openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        td.Name,
					Description: td.Description,
					Parameters:  td.Parameters,
				},
			})
		}
		body.Tools = oaiTools
		body.ToolChoice = "auto"
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openclaw: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openclaw: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	// Agent routing: select which OpenClaw agent handles this request
	httpReq.Header.Set("x-openclaw-agent-id", c.cfg.ExternalAgentId)
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openclaw: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openclaw: HTTP %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan LLMChunk, 64)
	// Reuse the OpenAI SSE parser — same format
	oai := &openAIClient{cfg: c.cfg, httpClient: c.httpClient}
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		oai.parseSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// ---------------------------------------------------------------------------
// Anthropic adapter
// ---------------------------------------------------------------------------

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	defaultAnthropicModel   = "claude-sonnet-4-6"
	anthropicVersion        = "2023-06-01"
)

type anthropicClient struct {
	cfg        LLMServiceConfig
	httpClient *http.Client
}

func newAnthropicClient(cfg LLMServiceConfig) *anthropicClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultAnthropicBaseURL
	}
	return &anthropicClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *anthropicClient) ProviderName() string { return "anthropic" }

// anthropicMessagesRequest matches the Anthropic Messages API request body.
type anthropicMessagesRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []anthropicMsg  `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Tools       []anthropicTool `json:"tools,omitempty"`
}

// anthropicMsg supports both simple string content and multi-block content.
// When Content is non-empty it is used; when ContentBlocks is non-empty that is used instead.
type anthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []interface{}
}

// anthropicTool describes a callable tool for Anthropic's API.
type anthropicTool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ToolSchema `json:"input_schema"`
}

// anthropicToolUseBlock represents a tool_use content block in an assistant message.
type anthropicToolUseBlock struct {
	Type  string          `json:"type"`
	Id    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// anthropicContentBlock is used in SSE content_block_start events.
type anthropicContentBlock struct {
	Type string `json:"type"`
	Id   string `json:"id"`
	Name string `json:"name"`
}

// anthropicSSEEvent represents a parsed Server-Sent Event from the Anthropic API.
type anthropicSSEEvent struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *anthropicDelta        `json:"delta,omitempty"`
	Usage        *anthropicUsage        `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJson string `json:"partial_json"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (c *anthropicClient) Stream(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	modelId := req.ModelId
	if modelId == "" {
		modelId = defaultAnthropicModel
	}
	maxTokens := 8096
	if req.Parameters.MaxTokens != nil {
		maxTokens = *req.Parameters.MaxTokens
	}

	msgs := make([]anthropicMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue // system goes in the top-level System field
		}

		// role=tool: Anthropic expects a "user" message with a tool_result content block
		if m.Role == "tool" {
			block := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallId,
				"content":     m.Content,
			}
			msgs = append(msgs, anthropicMsg{
				Role:    "user",
				Content: []interface{}{block},
			})
			continue
		}

		// role=assistant with tool calls: multi-block content
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var blocks []interface{}
			if m.Content != "" {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var inputRaw json.RawMessage
				if tc.Input != "" {
					inputRaw = json.RawMessage(tc.Input)
				} else {
					inputRaw = json.RawMessage("{}")
				}
				blocks = append(blocks, anthropicToolUseBlock{
					Type:  "tool_use",
					Id:    tc.ID,
					Name:  tc.Name,
					Input: inputRaw,
				})
			}
			msgs = append(msgs, anthropicMsg{
				Role:    "assistant",
				Content: blocks,
			})
			continue
		}

		msgs = append(msgs, anthropicMsg{Role: m.Role, Content: m.Content})
	}

	body := anthropicMessagesRequest{
		Model:       modelId,
		MaxTokens:   maxTokens,
		System:      req.SystemPrompt,
		Messages:    msgs,
		Stream:      true,
		Temperature: req.Parameters.Temperature,
		TopP:        req.Parameters.TopP,
	}

	if len(req.Tools) > 0 {
		aTools := make([]anthropicTool, 0, len(req.Tools))
		for _, td := range req.Tools {
			aTools = append(aTools, anthropicTool{
				Name:        td.Name,
				Description: td.Description,
				InputSchema: td.Parameters,
			})
		}
		body.Tools = aTools
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan LLMChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		c.parseSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

func (c *anthropicClient) parseSSEStream(ctx context.Context, body io.Reader, ch chan<- LLMChunk) {
	var inputTokens, outputTokens int

	// Track tool_use content blocks by index
	type toolUseBlock struct {
		id          string
		name        string
		inputBuffer strings.Builder
	}
	toolBlocks := map[int]*toolUseBlock{}

	scanner := bufio.NewScanner(body)
	var eventType string
	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- LLMChunk{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			eventData.WriteString(data)
			continue
		}

		if line == "" && eventData.Len() > 0 {
			// Parse accumulated event
			var evt anthropicSSEEvent
			if err := json.Unmarshal([]byte(eventData.String()), &evt); err == nil {
				// Use the event type from the "event:" line if available, else from JSON
				evtType := eventType
				if evtType == "" {
					evtType = evt.Type
				}

				switch evtType {
				case "message_start":
					if evt.Usage != nil {
						inputTokens = evt.Usage.InputTokens
					}

				case "content_block_start":
					if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
						toolBlocks[evt.Index] = &toolUseBlock{
							id:   evt.ContentBlock.Id,
							name: evt.ContentBlock.Name,
						}
					}

				case "content_block_delta":
					if evt.Delta != nil {
						switch evt.Delta.Type {
						case "text_delta":
							ch <- LLMChunk{Text: evt.Delta.Text}
						case "input_json_delta":
							if tb, ok := toolBlocks[evt.Index]; ok {
								tb.inputBuffer.WriteString(evt.Delta.PartialJson)
							}
						}
					}

				case "content_block_stop":
					// Nothing special needed; tool block is finalized at message_stop

				case "message_delta":
					if evt.Usage != nil {
						outputTokens = evt.Usage.OutputTokens
					}

				case "message_stop":
					// Collect any completed tool calls
					if len(toolBlocks) > 0 {
						toolCalls := make([]LLMToolCall, 0, len(toolBlocks))
						for _, tb := range toolBlocks {
							toolCalls = append(toolCalls, LLMToolCall{
								ID:    tb.id,
								Name:  tb.name,
								Input: tb.inputBuffer.String(),
							})
						}
						ch <- LLMChunk{
							Done:         true,
							InputTokens:  inputTokens,
							OutputTokens: outputTokens,
							ToolCalls:    toolCalls,
						}
						return
					}
				}
			}
			eventData.Reset()
			eventType = ""
		}
	}

	ch <- LLMChunk{Done: true, InputTokens: inputTokens, OutputTokens: outputTokens}
}

// ---------------------------------------------------------------------------
// OpenAI adapter
// ---------------------------------------------------------------------------

const (
	defaultOpenAIBaseURL = "https://api.openai.com"
	defaultOpenAIModel   = "gpt-4o"
)

type openAIClient struct {
	cfg        LLMServiceConfig
	httpClient *http.Client
}

func newOpenAIClient(cfg LLMServiceConfig) *openAIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultOpenAIBaseURL
	}
	return &openAIClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *openAIClient) ProviderName() string { return "openai" }

type openAIChatRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMsg       `json:"messages"`
	Stream      bool              `json:"stream"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	StreamOpts  *openAIStreamOpts `json:"stream_options,omitempty"`
	Tools       []openAITool      `json:"tools,omitempty"`
	ToolChoice  string            `json:"tool_choice,omitempty"`
}

type openAIStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMsg struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallId string           `json:"tool_call_id,omitempty"`
}

// openAITool describes a callable function tool for the OpenAI API.
type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

// openAIFunction is the function definition inside an openAITool.
type openAIFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  ToolSchema `json:"parameters"`
}

// openAIToolCall represents a tool call in an assistant message or a streaming delta.
type openAIToolCall struct {
	Id       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

// openAIToolCallFunction holds the name and arguments for a tool call.
type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIUsage         `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type openAIDelta struct {
	Content   string                `json:"content"`
	Role      string                `json:"role"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

// openAIToolCallDelta is a streaming partial tool call from the OpenAI API.
type openAIToolCallDelta struct {
	Index    int                         `json:"index"`
	Id       string                      `json:"id"`
	Type     string                      `json:"type"`
	Function openAIToolCallFunctionDelta `json:"function"`
}

// openAIToolCallFunctionDelta carries partial name/arguments fragments.
type openAIToolCallFunctionDelta struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (c *openAIClient) Stream(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	modelId := req.ModelId
	if modelId == "" {
		modelId = defaultOpenAIModel
	}

	msgs := make([]openAIMsg, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, openAIMsg{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		if m.Role == "tool" {
			msgs = append(msgs, openAIMsg{
				Role:       "tool",
				Content:    m.Content,
				ToolCallId: m.ToolCallId,
			})
			continue
		}
		if len(m.ToolCalls) > 0 {
			oaiToolCalls := make([]openAIToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				oaiToolCalls = append(oaiToolCalls, openAIToolCall{
					Id:   tc.ID,
					Type: "function",
					Function: openAIToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
			msgs = append(msgs, openAIMsg{
				Role:      m.Role,
				Content:   m.Content,
				ToolCalls: oaiToolCalls,
			})
			continue
		}
		msgs = append(msgs, openAIMsg{Role: m.Role, Content: m.Content})
	}

	body := openAIChatRequest{
		Model:       modelId,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   req.Parameters.MaxTokens,
		Temperature: req.Parameters.Temperature,
		TopP:        req.Parameters.TopP,
		StreamOpts:  &openAIStreamOpts{IncludeUsage: true},
	}

	if len(req.Tools) > 0 {
		oaiTools := make([]openAITool, 0, len(req.Tools))
		for _, td := range req.Tools {
			oaiTools = append(oaiTools, openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        td.Name,
					Description: td.Description,
					Parameters:  td.Parameters,
				},
			})
		}
		body.Tools = oaiTools
		body.ToolChoice = "auto"
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan LLMChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		c.parseSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

func (c *openAIClient) parseSSEStream(ctx context.Context, body io.Reader, ch chan<- LLMChunk) {
	var inputTokens, outputTokens int

	// Accumulate streaming tool call fragments by index
	type toolCallAccum struct {
		id      string
		name    string
		argsBuf strings.Builder
	}
	toolAccums := map[int]*toolCallAccum{}
	var finishReason string

	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- LLMChunk{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
		}

		for _, choice := range chunk.Choices {
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = *choice.FinishReason
			}

			if choice.Delta.Content != "" {
				ch <- LLMChunk{Text: choice.Delta.Content}
			}

			// Accumulate tool call deltas
			for _, tcd := range choice.Delta.ToolCalls {
				accum, ok := toolAccums[tcd.Index]
				if !ok {
					accum = &toolCallAccum{}
					toolAccums[tcd.Index] = accum
				}
				if tcd.Id != "" {
					accum.id = tcd.Id
				}
				if tcd.Function.Name != "" {
					accum.name = tcd.Function.Name
				}
				if tcd.Function.Arguments != "" {
					accum.argsBuf.WriteString(tcd.Function.Arguments)
				}
			}
		}
	}

	// If finish_reason was "tool_calls", emit accumulated tool calls in the Done chunk
	if finishReason == "tool_calls" && len(toolAccums) > 0 {
		toolCalls := make([]LLMToolCall, 0, len(toolAccums))
		for _, accum := range toolAccums {
			toolCalls = append(toolCalls, LLMToolCall{
				ID:    accum.id,
				Name:  accum.name,
				Input: accum.argsBuf.String(),
			})
		}
		ch <- LLMChunk{
			Done:         true,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			ToolCalls:    toolCalls,
		}
		return
	}

	ch <- LLMChunk{Done: true, InputTokens: inputTokens, OutputTokens: outputTokens}
}
