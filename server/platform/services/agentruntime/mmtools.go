// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime – mmtools.go
// Built-in tools that let agents interact with the Mattermost workspace.
// Each tool satisfies the AgentTool interface.

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// registerBuiltinTools adds all built-in Mattermost tools to the registry.
func registerBuiltinTools(reg *ToolRegistry, app AgentAppIface) {
	reg.Register(&searchPostsTool{app: app})
	reg.Register(&getChannelTool{app: app})
	reg.Register(&createPostTool{app: app})
	reg.Register(&getUserTool{app: app})
	reg.Register(&listChannelsTool{app: app})
}

// ---------------------------------------------------------------------------
// search_posts
// ---------------------------------------------------------------------------

type searchPostsTool struct{ app AgentAppIface }

func (t *searchPostsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_posts",
		Description: "Search for posts in the current team matching a query string. Returns up to 20 matching posts with their author, channel, and message text.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"query": {
					Type:        "string",
					Description: "Full-text search query (supports Mattermost search modifiers like from:, in:, before:, after:)",
				},
				"team_id": {
					Type:        "string",
					Description: "Team ID to search within (required)",
				},
			},
			Required: []string{"query", "team_id"},
		},
	}
}

func (t *searchPostsTool) Execute(_ context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		Query  string `json:"query"`
		TeamId string `json:"team_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("search_posts: invalid args: %w", err)
	}
	if params.Query == "" {
		return "", fmt.Errorf("search_posts: query is required")
	}

	searchParams := model.ParseSearchParams(params.Query, 0)
	posts, appErr := t.app.SearchPostsInTeam(params.TeamId, searchParams)
	if appErr != nil {
		return "", fmt.Errorf("search_posts: %s", appErr.Error())
	}

	if posts == nil || len(posts.Posts) == 0 {
		return "No posts found matching the query.", nil
	}

	var sb strings.Builder
	count := 0
	for _, p := range posts.Posts {
		if count >= 20 {
			break
		}
		fmt.Fprintf(&sb, "- [%s] %s\n", p.Id[:8], p.Message)
		count++
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// get_channel
// ---------------------------------------------------------------------------

type getChannelTool struct{ app AgentAppIface }

func (t *getChannelTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_channel",
		Description: "Look up a Mattermost channel by name or ID. Returns the channel's display name, type, purpose, and header.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"channel_id": {
					Type:        "string",
					Description: "Channel ID (use this OR channel_name+team_id)",
				},
				"channel_name": {
					Type:        "string",
					Description: "Channel name without the # prefix",
				},
				"team_id": {
					Type:        "string",
					Description: "Team ID (required when using channel_name)",
				},
			},
		},
	}
}

func (t *getChannelTool) Execute(_ context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		ChannelId   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		TeamId      string `json:"team_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("get_channel: invalid args: %w", err)
	}

	var channel *model.Channel
	var appErr *model.AppError

	if params.ChannelId != "" {
		channel, appErr = t.app.GetChannel(rctx, params.ChannelId)
	} else if params.ChannelName != "" && params.TeamId != "" {
		channel, appErr = t.app.GetChannelByName(rctx, params.ChannelName, params.TeamId, false)
	} else {
		return "", fmt.Errorf("get_channel: provide channel_id or channel_name+team_id")
	}

	if appErr != nil {
		return "", fmt.Errorf("get_channel: %s", appErr.Error())
	}

	result := fmt.Sprintf("Name: %s\nDisplay name: %s\nType: %s\nPurpose: %s\nHeader: %s\nID: %s",
		channel.Name, channel.DisplayName, channel.Type,
		channel.Purpose, channel.Header, channel.Id)
	return result, nil
}

// ---------------------------------------------------------------------------
// create_post
// ---------------------------------------------------------------------------

type createPostTool struct{ app AgentAppIface }

func (t *createPostTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_post",
		Description: "Post a message to a Mattermost channel or reply in a thread.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"channel_id": {
					Type:        "string",
					Description: "Channel ID to post in",
				},
				"message": {
					Type:        "string",
					Description: "Markdown message body",
				},
				"root_id": {
					Type:        "string",
					Description: "Root post ID to reply to (optional — omit for new top-level post)",
				},
				"bot_user_id": {
					Type:        "string",
					Description: "Bot user ID that should author the post",
				},
			},
			Required: []string{"channel_id", "message", "bot_user_id"},
		},
	}
}

func (t *createPostTool) Execute(_ context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		ChannelId string `json:"channel_id"`
		Message   string `json:"message"`
		RootId    string `json:"root_id"`
		BotUserId string `json:"bot_user_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("create_post: invalid args: %w", err)
	}
	if params.ChannelId == "" || params.Message == "" || params.BotUserId == "" {
		return "", fmt.Errorf("create_post: channel_id, message, and bot_user_id are required")
	}

	channel, chErr := t.app.GetChannel(rctx, params.ChannelId)
	if chErr != nil {
		return "", fmt.Errorf("create_post: get channel: %s", chErr.Error())
	}

	post := &model.Post{
		UserId:    params.BotUserId,
		ChannelId: params.ChannelId,
		Message:   params.Message,
		RootId:    params.RootId,
		Props: model.StringInterface{
			"from_agent": true,
		},
	}

	created, _, appErr := t.app.CreatePost(rctx, post, channel, model.CreatePostFlags{})
	if appErr != nil {
		return "", fmt.Errorf("create_post: %s", appErr.Error())
	}

	return fmt.Sprintf("Post created: %s", created.Id), nil
}

// ---------------------------------------------------------------------------
// get_user
// ---------------------------------------------------------------------------

type getUserTool struct{ app AgentAppIface }

func (t *getUserTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_user",
		Description: "Look up a Mattermost user by username or ID. Returns display name, username, email, and role.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"username": {
					Type:        "string",
					Description: "Username without the @ prefix (use this OR user_id)",
				},
				"user_id": {
					Type:        "string",
					Description: "User ID",
				},
			},
		},
	}
}

func (t *getUserTool) Execute(_ context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		Username string `json:"username"`
		UserId   string `json:"user_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("get_user: invalid args: %w", err)
	}

	var user *model.User
	var appErr *model.AppError

	if params.UserId != "" {
		user, appErr = t.app.GetUser(params.UserId)
	} else if params.Username != "" {
		user, appErr = t.app.GetUserByUsername(params.Username)
	} else {
		return "", fmt.Errorf("get_user: provide username or user_id")
	}

	if appErr != nil {
		return "", fmt.Errorf("get_user: %s", appErr.Error())
	}

	return fmt.Sprintf("Username: %s\nDisplay name: %s\nEmail: %s\nRoles: %s\nID: %s",
		user.Username, user.GetDisplayName("full_name"),
		user.Email, user.Roles, user.Id), nil
}

// ---------------------------------------------------------------------------
// list_channels
// ---------------------------------------------------------------------------

type listChannelsTool struct{ app AgentAppIface }

func (t *listChannelsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_channels",
		Description: "List public channels in a team. Returns name, display name, purpose, and member count.",
		Parameters: ToolSchema{
			Type: "object",
			Properties: map[string]*ToolSchema{
				"team_id": {
					Type:        "string",
					Description: "Team ID to list channels for (required)",
				},
				"page": {
					Type:        "integer",
					Description: "Page number (0-indexed, default 0)",
				},
				"per_page": {
					Type:        "integer",
					Description: "Results per page (default 20, max 50)",
				},
			},
			Required: []string{"team_id"},
		},
	}
}

func (t *listChannelsTool) Execute(_ context.Context, rctx request.CTX, args json.RawMessage) (string, error) {
	var params struct {
		TeamId  string `json:"team_id"`
		Page    int    `json:"page"`
		PerPage int    `json:"per_page"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("list_channels: invalid args: %w", err)
	}
	if params.TeamId == "" {
		return "", fmt.Errorf("list_channels: team_id is required")
	}
	if params.PerPage <= 0 || params.PerPage > 50 {
		params.PerPage = 20
	}

	channels, appErr := t.app.GetPublicChannelsForTeam(rctx, params.TeamId, params.Page*params.PerPage, params.PerPage)
	if appErr != nil {
		return "", fmt.Errorf("list_channels: %s", appErr.Error())
	}

	if len(channels) == 0 {
		return "No public channels found.", nil
	}

	var sb strings.Builder
	for _, ch := range channels {
		fmt.Fprintf(&sb, "- #%s (%s) — %s\n", ch.Name, ch.Id[:8], ch.Purpose)
	}
	return sb.String(), nil
}
