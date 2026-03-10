// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

type mockAgentApp struct {
	mock.Mock
}

func (m *mockAgentApp) CreatePost(rctx request.CTX, post *model.Post, channel *model.Channel, flags model.CreatePostFlags) (*model.Post, bool, *model.AppError) {
	args := m.Called(rctx, post, channel, flags)
	if args.Get(0) == nil {
		return nil, false, args.Get(2).(*model.AppError)
	}
	return args.Get(0).(*model.Post), args.Bool(1), nil
}
func (m *mockAgentApp) PatchPost(rctx request.CTX, postID string, patch *model.PostPatch, opts *model.UpdatePostOptions) (*model.Post, bool, *model.AppError) {
	return nil, false, nil
}
func (m *mockAgentApp) GetChannel(rctx request.CTX, channelID string) (*model.Channel, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) GetChannelByName(rctx request.CTX, channelName, teamID string, includeDeleted bool) (*model.Channel, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) GetPostThread(rctx request.CTX, postID string, opts model.GetPostsOptions, userID string) (*model.PostList, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) GetPosts(rctx request.CTX, channelID string, offset int, limit int) (*model.PostList, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) GetPublicChannelsForTeam(rctx request.CTX, teamID string, offset int, limit int) (model.ChannelList, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) GetUser(userID string) (*model.User, *model.AppError) { return nil, nil }
func (m *mockAgentApp) GetUserByUsername(username string) (*model.User, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) SearchPostsInTeam(teamID string, paramsList []*model.SearchParams) (*model.PostList, *model.AppError) {
	return nil, nil
}
func (m *mockAgentApp) CreatePostMissingChannel(rctx request.CTX, post *model.Post, triggerWebhooks bool, setOnline bool) (*model.Post, bool, *model.AppError) {
	args := m.Called(rctx, post, triggerWebhooks, setOnline)
	if args.Get(0) == nil {
		return nil, false, args.Get(2).(*model.AppError)
	}
	return args.Get(0).(*model.Post), args.Bool(1), nil
}
func (m *mockAgentApp) Publish(message *model.WebSocketEvent) {}

// ---------------------------------------------------------------------------
// delegateTaskTool Definition test
// ---------------------------------------------------------------------------

func TestDelegateTaskTool_Definition(t *testing.T) {
	tool := &delegateTaskTool{}
	def := tool.Definition()
	assert.Equal(t, "delegate_task", def.Name)
	require.NotNil(t, def.Parameters.Properties["agent_id"])
	require.NotNil(t, def.Parameters.Properties["instructions"])
	assert.Contains(t, def.Parameters.Required, "agent_id")
	assert.Contains(t, def.Parameters.Required, "instructions")
}

func TestRouteToCapabilityTool_Definition(t *testing.T) {
	tool := &routeToCapabilityTool{}
	def := tool.Definition()
	assert.Equal(t, "route_to_capability", def.Name)
	require.NotNil(t, def.Parameters.Properties["capabilities"])
	require.NotNil(t, def.Parameters.Properties["instructions"])
}

// ---------------------------------------------------------------------------
// postToCoordinationChannel: same-workgroup delegation skips coordination post
// ---------------------------------------------------------------------------

func TestPostToCoordinationChannel_SameWorkgroup_Skips(t *testing.T) {
	agentStore := mocks.NewAgentStore(t)
	st := mocks.NewStore(t)
	st.On("Agent").Return(agentStore).Maybe()

	wgId := model.NewId()
	callerDefId := model.NewId()
	targetDefId := model.NewId()

	callerDef := &model.AgentDefinition{Id: callerDefId, WorkgroupId: wgId, DisplayName: "Caller"}
	targetDef := &model.AgentDefinition{Id: targetDefId, WorkgroupId: wgId, DisplayName: "Target"}

	agentStore.On("GetAgentDefinition", callerDefId).Return(callerDef, nil).Maybe()

	app := &mockAgentApp{}
	tool := &delegateTaskTool{store: st, app: app}

	ctx := context.WithValue(context.Background(), contextKeyAgentID, callerDefId)
	subtask := &model.AgentTask{Id: model.NewId()}

	// Should not call CreatePostMissingChannel because same workgroup
	tool.postToCoordinationChannel(ctx, request.EmptyContext(mlog.CreateConsoleTestLogger(t)),
		targetDefId, targetDef, "Target", subtask, "do something")

	app.AssertNotCalled(t, "CreatePostMissingChannel")
}

// ---------------------------------------------------------------------------
// postToCoordinationChannel: no context agent ID skips gracefully
// ---------------------------------------------------------------------------

func TestPostToCoordinationChannel_NoAgentCtx_Skips(t *testing.T) {
	app := &mockAgentApp{}
	tool := &delegateTaskTool{app: app}

	targetDef := &model.AgentDefinition{Id: model.NewId(), WorkgroupId: model.NewId()}
	subtask := &model.AgentTask{Id: model.NewId()}

	// context has no agent ID — should be a no-op
	tool.postToCoordinationChannel(context.Background(),
		request.EmptyContext(mlog.CreateConsoleTestLogger(t)),
		targetDef.Id, targetDef, "Target", subtask, "do something")

	app.AssertNotCalled(t, "CreatePostMissingChannel")
}

// ---------------------------------------------------------------------------
// postToCoordinationChannel: cross-workgroup posts to coordination channel
// ---------------------------------------------------------------------------

func TestPostToCoordinationChannel_CrossWorkgroup_Posts(t *testing.T) {
	agentStore := mocks.NewAgentStore(t)
	st := mocks.NewStore(t)
	st.On("Agent").Return(agentStore).Maybe()

	callerWgId := model.NewId()
	targetWgId := model.NewId()
	coordChId := model.NewId()
	callerDefId := model.NewId()
	targetDefId := model.NewId()

	callerDef := &model.AgentDefinition{Id: callerDefId, WorkgroupId: callerWgId, DisplayName: "CEO"}
	targetDef := &model.AgentDefinition{Id: targetDefId, WorkgroupId: targetWgId, DisplayName: "Sales Head"}

	callerWg := &model.Workgroup{Id: callerWgId}
	callerWg.ChannelIds.SetCoordinationChannel(targetWgId, coordChId)

	agentStore.On("GetAgentDefinition", callerDefId).Return(callerDef, nil)
	agentStore.On("GetWorkgroup", callerWgId).Return(callerWg, nil)

	app := &mockAgentApp{}
	app.On("CreatePostMissingChannel",
		mock.Anything,
		mock.MatchedBy(func(p *model.Post) bool { return p.ChannelId == coordChId }),
		false, false,
	).Return(&model.Post{Id: model.NewId()}, false, nil)

	tool := &delegateTaskTool{store: st, app: app}
	ctx := context.WithValue(context.Background(), contextKeyAgentID, callerDefId)
	subtask := &model.AgentTask{Id: model.NewId()}

	tool.postToCoordinationChannel(ctx,
		request.EmptyContext(mlog.CreateConsoleTestLogger(t)),
		targetDefId, targetDef, "Sales Head", subtask, "prepare Q3 sales forecast")

	app.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// delegateTaskTool Execute: missing required params returns error
// ---------------------------------------------------------------------------

func TestDelegateTaskTool_Execute_MissingParams(t *testing.T) {
	tool := &delegateTaskTool{}
	args, _ := json.Marshal(map[string]string{"agent_id": ""})
	_, err := tool.Execute(context.Background(), request.EmptyContext(mlog.CreateConsoleTestLogger(t)), args)
	require.Error(t, err)
}

func TestDelegateTaskTool_Execute_BadJSON(t *testing.T) {
	tool := &delegateTaskTool{}
	_, err := tool.Execute(context.Background(), request.EmptyContext(mlog.CreateConsoleTestLogger(t)), json.RawMessage(`{bad json}`))
	require.Error(t, err)
}
