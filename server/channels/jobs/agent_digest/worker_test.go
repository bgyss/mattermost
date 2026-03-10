// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agent_digest

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockAppIface struct{ mock.Mock }

func (m *mockAppIface) CreatePostMissingChannel(rctx request.CTX, post *model.Post, triggerWebhooks bool, setOnline bool) (*model.Post, bool, *model.AppError) {
	args := m.Called(rctx, post, triggerWebhooks, setOnline)
	if args.Get(0) == nil {
		return nil, false, args.Get(2).(*model.AppError)
	}
	return args.Get(0).(*model.Post), args.Bool(1), nil
}

type mockMetrics struct{ data []AgentMetrics }

func (m *mockMetrics) GetMetrics() []AgentMetrics { return m.data }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestProcessDigestJob_NoChannelId(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	job := &model.Job{Data: map[string]string{}}
	app := &mockAppIface{}
	metrics := &mockMetrics{}

	err := processDigestJob(logger, job, app, metrics)
	require.NoError(t, err, "missing channel_id should be a no-op, not an error")
	app.AssertNotCalled(t, "CreatePostMissingChannel")
}

func TestProcessDigestJob_NoMetrics(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	job := &model.Job{Data: map[string]string{
		"channel_id":  "ch1",
		"bot_user_id": "bot1",
	}}
	app := &mockAppIface{}
	metrics := &mockMetrics{data: []AgentMetrics{}} // empty

	err := processDigestJob(logger, job, app, metrics)
	require.NoError(t, err, "no metrics → no-op, not error")
	app.AssertNotCalled(t, "CreatePostMissingChannel")
}

func TestProcessDigestJob_PostsDigest(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	job := &model.Job{Data: map[string]string{
		"channel_id":  "ch1",
		"bot_user_id": "bot1",
	}}
	app := &mockAppIface{}
	metrics := &mockMetrics{data: []AgentMetrics{
		{AgentId: "aaa111", TasksSubmitted: 10, TasksCompleted: 8, TasksFailed: 2, TokensTotal: 5000, AvgLatencyMs: 1200},
	}}

	app.On("CreatePostMissingChannel",
		mock.Anything,
		mock.MatchedBy(func(p *model.Post) bool {
			return p.ChannelId == "ch1" && p.UserId == "bot1"
		}),
		false, false,
	).Return(&model.Post{Id: model.NewId()}, false, nil)

	err := processDigestJob(logger, job, app, metrics)
	require.NoError(t, err)
	app.AssertExpectations(t)
}

func TestProcessDigestJob_PostContentContainsMetrics(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	job := &model.Job{Data: map[string]string{
		"channel_id":  "ch1",
		"bot_user_id": "bot1",
	}}
	app := &mockAppIface{}
	metrics := &mockMetrics{data: []AgentMetrics{
		{AgentId: "agentXYZ123", TasksSubmitted: 5, TasksCompleted: 4, TasksFailed: 1, TokensTotal: 1500, AvgLatencyMs: 800},
	}}

	var capturedPost *model.Post
	app.On("CreatePostMissingChannel", mock.Anything, mock.Anything, false, false).
		Run(func(args mock.Arguments) {
			capturedPost = args.Get(1).(*model.Post)
		}).
		Return(&model.Post{Id: model.NewId()}, false, nil)

	err := processDigestJob(logger, job, app, metrics)
	require.NoError(t, err)
	require.NotNil(t, capturedPost)
	assert.Contains(t, capturedPost.Message, "Agent Activity Digest")
	assert.Contains(t, capturedPost.Message, "agentXYZ")
}

// ---------------------------------------------------------------------------
// formatDigest / formatTokens unit tests
// ---------------------------------------------------------------------------

func TestFormatDigest_ContainsExpectedColumns(t *testing.T) {
	metrics := []AgentMetrics{
		{AgentId: "abc123def456", TasksSubmitted: 10, TasksCompleted: 9, TasksFailed: 1, TokensTotal: 1_500_000, AvgLatencyMs: 300},
	}
	out := formatDigest(metrics)
	assert.Contains(t, out, "Submitted")
	assert.Contains(t, out, "Completed")
	assert.Contains(t, out, "Failed")
	assert.Contains(t, out, "Tokens")
	assert.Contains(t, out, "1.5M")
	assert.Contains(t, out, "300ms")
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, formatTokens(tc.n), "n=%d", tc.n)
	}
}
