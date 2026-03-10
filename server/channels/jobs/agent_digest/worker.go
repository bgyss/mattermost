// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agent_digest provides a background job that posts hourly activity
// digests from each active agent to their department's #reports channel.

package agent_digest

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
)

// AgentMetricsProvider is the subset of AgentRuntimeService the digest job needs.
type AgentMetricsProvider interface {
	GetMetrics() []AgentMetrics
}

// AgentMetrics mirrors agentruntime.AgentMetrics without importing the package
// to avoid a circular dependency. The JSON tags must match.
type AgentMetrics struct {
	AgentId        string `json:"agent_id"`
	TasksSubmitted int64  `json:"tasks_submitted"`
	TasksCompleted int64  `json:"tasks_completed"`
	TasksFailed    int64  `json:"tasks_failed"`
	TokensTotal    int64  `json:"tokens_total"`
	AvgLatencyMs   int64  `json:"avg_latency_ms"`
}

// AppIface is the subset of app.App the digest job needs.
type AppIface interface {
	CreatePostMissingChannel(rctx request.CTX, post *model.Post, triggerWebhooks bool, setOnline bool) (*model.Post, bool, *model.AppError)
}

// MakeWorker returns a SimpleWorker that posts digests to the designated channel.
// The job's Data map must contain:
//   - "channel_id"  – channel to post the digest to
//   - "bot_user_id" – bot user ID to post as
func MakeWorker(jobServer *jobs.JobServer, appInstance AppIface, metrics AgentMetricsProvider) *jobs.SimpleWorker {
	isEnabled := func(cfg *model.Config) bool {
		return cfg.FeatureFlags.EnableAIAgents
	}

	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		defer jobServer.HandleJobPanic(logger, job)
		return processDigestJob(logger, job, appInstance, metrics)
	}

	return jobs.NewSimpleWorker("AgentDigest", jobServer, execute, isEnabled)
}

func processDigestJob(logger mlog.LoggerIFace, job *model.Job, appInstance AppIface, metrics AgentMetricsProvider) error {
	channelId := job.Data["channel_id"]
	botUserId := job.Data["bot_user_id"]
	if channelId == "" || botUserId == "" {
		logger.Warn("agent_digest: missing channel_id or bot_user_id in job data")
		return nil
	}

	snapshot := metrics.GetMetrics()
	if len(snapshot) == 0 {
		return nil // nothing to report
	}

	msg := formatDigest(snapshot)
	rctx := request.EmptyContext(logger)

	post := &model.Post{
		ChannelId: channelId,
		UserId:    botUserId,
		Message:   msg,
		Type:      model.PostTypeDefault,
	}
	if _, _, appErr := appInstance.CreatePostMissingChannel(rctx, post, false, false); appErr != nil {
		logger.Warn("agent_digest: failed to post digest", mlog.Err(appErr))
	}
	return nil
}

func formatDigest(metrics []AgentMetrics) string {
	var sb strings.Builder
	sb.WriteString("### Agent Activity Digest\n\n")
	sb.WriteString("| Agent | Submitted | Completed | Failed | Tokens | Avg Latency |\n")
	sb.WriteString("|-------|-----------|-----------|--------|--------|-------------|\n")
	for _, m := range metrics {
		sb.WriteString(fmt.Sprintf("| `%s` | %d | %d | %d | %s | %dms |\n",
			m.AgentId[:min(8, len(m.AgentId))],
			m.TasksSubmitted,
			m.TasksCompleted,
			m.TasksFailed,
			formatTokens(m.TokensTotal),
			m.AvgLatencyMs,
		))
	}
	return sb.String()
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
