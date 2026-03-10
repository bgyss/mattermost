// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

var mentionRE = regexp.MustCompile(`@([a-zA-Z0-9_.\-]+)`)

// HandleAgentPostHook is called after every post is created.
// It detects @mentions of agent bot users and submits tasks to the AgentRuntimeService.
func (a *App) HandleAgentPostHook(rctx request.CTX, post *model.Post) {
	ars := a.AgentRuntimeService()
	if ars == nil {
		return
	}

	// Skip system posts to avoid noise
	if post.Type != "" {
		return
	}

	channel, chanErr := a.GetChannel(rctx, post.ChannelId)
	if chanErr != nil {
		return
	}

	// Collect bot user IDs to try: all @mentioned usernames + DM partner if DM channel
	botUserIds := a.extractAgentBotUserIds(rctx, post, channel)

	for _, botUserId := range botUserIds {
		// Skip if the post is from this agent (prevent feedback loop)
		if post.UserId == botUserId {
			continue
		}

		def, storeErr := a.Srv().Store().Agent().GetAgentDefinitionByBotUserId(botUserId)
		if storeErr != nil {
			// Not an agent bot user — skip silently
			continue
		}

		threadRootId := post.RootId
		if threadRootId == "" {
			threadRootId = post.Id
		}

		req := &model.SubmitTaskRequest{
			AgentId:          def.Id,
			ChannelId:        post.ChannelId,
			RequestPostId:    post.Id,
			ThreadRootPostId: threadRootId,
			Text:             post.Message,
			Priority:         model.AgentTaskPriorityNormal,
		}

		if _, submitErr := ars.SubmitTask(rctx, req); submitErr != nil {
			rctx.Logger().Warn("HandleAgentPostHook: failed to submit task",
				mlog.String("agent_id", def.Id),
				mlog.String("post_id", post.Id),
				mlog.Err(submitErr),
			)
		}
	}
}

// extractAgentBotUserIds returns a deduplicated list of user IDs to check for agent definitions.
// For @mentions: resolves username → userId.
// For DM channels: includes the non-poster member.
func (a *App) extractAgentBotUserIds(rctx request.CTX, post *model.Post, channel *model.Channel) []string {
	seen := make(map[string]bool)
	var ids []string

	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	// Extract @mentions from post text
	for _, match := range mentionRE.FindAllStringSubmatch(post.Message, -1) {
		username := match[1]
		user, err := a.GetUserByUsername(username)
		if err != nil {
			continue
		}
		if user.IsBot {
			add(user.Id)
		}
	}

	// For DM channels, always include the other party
	if channel.Type == model.ChannelTypeDirect {
		// DM channel name format: "userId1__userId2" — get both and add the non-poster
		parts := strings.Split(channel.Name, "__")
		for _, part := range parts {
			if part != "" && part != post.UserId {
				add(part)
			}
		}
	}

	return ids
}
