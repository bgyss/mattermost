// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Package agentruntime provides the core execution engine for autonomous AI agents.
// It is responsible for dispatching tasks, executing LLM loops with tool calling,
// managing agent memory, and emitting observability events.
//
// Architecture:
//   - Service           – lifecycle management (Start/Stop), wires sub-components
//   - Dispatcher        – priority queue, per-agent semaphores, cluster-safe claiming
//   - Executor          – core agentic loop: context build → LLM → tools → stream
//   - CapabilityRouter  – routes tasks to least-loaded agents by capability tag
//   - ToolRegistry      – AgentTool interface + built-in mmtools + delegation tools
//   - LLMService        – abstraction over OpenClaw (default), Anthropic, OpenAI
//   - Observability     – TaskEvent emission, metrics counters
package agentruntime

import (
	"context"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// ServerIface is the subset of app.Server that the runtime needs.
type ServerIface interface {
	Store() store.Store
	Log() *mlog.Logger
	ServerId() string
}

// AgentRuntimeServiceIFace is the public contract for the agent runtime.
// This interface is what the rest of the server references to avoid tight coupling.
type AgentRuntimeServiceIFace interface {
	// SubmitTask queues a new agent task and returns it (persisted, pending).
	SubmitTask(rctx request.CTX, req *model.SubmitTaskRequest) (*model.AgentTask, error)

	// GetTask returns the current state of a task.
	GetTask(rctx request.CTX, taskId string) (*model.AgentTask, error)

	// GetTaskTree returns the full delegation DAG for a root task.
	GetTaskTree(rctx request.CTX, rootTaskId string) ([]*model.AgentTask, error)

	// GetTaskEvents returns observability events for a task.
	GetTaskEvents(rctx request.CTX, taskId string, page, perPage int) ([]*model.AgentTaskEvent, error)

	// GetActiveTasksForAgent returns all non-terminal tasks for an agent.
	GetActiveTasksForAgent(rctx request.CTX, agentId string) ([]*model.AgentTask, error)

	// GetMetrics returns a snapshot of in-memory per-agent metrics.
	GetMetrics() []AgentMetrics

	// RegisterAgentForRouting adds an agent definition to the capability router.
	// Called by the workgroup provisioner after creating each agent.
	RegisterAgentForRouting(def *model.AgentDefinition)
}

// AgentRuntimeService is the concrete implementation of AgentRuntimeServiceIFace.
type AgentRuntimeService struct {
	srv        ServerIface
	store      store.Store
	log        *mlog.Logger
	dispatcher *dispatcher
	toolReg    *ToolRegistry
	router     *CapabilityRouter
	memMgr     *MemoryManager
	obs        *observabilityService

	mu      sync.RWMutex
	stopped bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewAgentRuntimeService creates and returns a new AgentRuntimeService.
// appIface must implement AgentAppIface (typically *app.App).
// Call Start() to begin processing.
func NewAgentRuntimeService(srv ServerIface, appIface AgentAppIface) (*AgentRuntimeService, error) {
	router := NewCapabilityRouter()
	toolReg := NewToolRegistry()
	memMgr := newMemoryManager(srv.Store(), srv.Log())
	obs := newObservabilityService()

	// We need a forward reference to svc for the delegation tools,
	// so build svc first and inject later.
	svc := &AgentRuntimeService{
		srv:     srv,
		store:   srv.Store(),
		log:     srv.Log(),
		stopCh:  make(chan struct{}),
		toolReg: toolReg,
		router:  router,
		memMgr:  memMgr,
		obs:     obs,
	}

	registerBuiltinTools(toolReg, appIface)
	registerMemoryTools(toolReg, memMgr)
	registerDelegationTools(toolReg, srv.Store(), appIface, router, svc)

	svc.dispatcher = newDispatcher(srv.Store(), appIface, srv.ServerId(), srv.Log(), toolReg, memMgr, obs)
	return svc, nil
}

// RegisterTool adds a custom tool to the registry. Must be called before Start().
func (s *AgentRuntimeService) RegisterTool(t AgentTool) {
	s.toolReg.Register(t)
}

// RegisterAgentForRouting adds an agent definition to the capability router so
// that route_to_capability can find it. Called after workgroup provisioning.
func (s *AgentRuntimeService) RegisterAgentForRouting(def *model.AgentDefinition) {
	s.router.Register(def)
}

// Start begins the background workers.
func (s *AgentRuntimeService) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil
	}

	s.log.Info("AgentRuntimeService starting")
	s.dispatcher.Start()

	// Background memory expiry cleanup.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-s.stopCh
			cancel()
		}()
		s.memMgr.runExpiryCleanup(ctx)
	}()

	return nil
}

// Stop shuts down all workers and waits for them to complete.
func (s *AgentRuntimeService) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stopCh)
	s.mu.Unlock()

	s.dispatcher.Stop()
	s.wg.Wait()
	s.log.Info("AgentRuntimeService stopped")
}

// ---------------------------------------------------------------------------
// AgentRuntimeServiceIFace implementation
// ---------------------------------------------------------------------------

// SubmitTask persists a new AgentTask and enqueues it for execution.
func (s *AgentRuntimeService) SubmitTask(rctx request.CTX, req *model.SubmitTaskRequest) (*model.AgentTask, error) {
	task := &model.AgentTask{
		AgentId:          req.AgentId,
		ChannelId:        req.ChannelId,
		RequestPostId:    req.RequestPostId,
		ThreadRootPostId: req.ThreadRootPostId,
		Input: model.AgentTaskInput{
			Text: req.Text,
		},
		Priority: req.Priority,
	}
	if task.Priority == 0 {
		task.Priority = model.AgentTaskPriorityNormal
	}

	saved, err := s.store.Agent().SaveAgentTask(task)
	if err != nil {
		return nil, err
	}
	s.obs.RecordSubmit(saved.AgentId)

	// Emit delegation WebSocket event if this is a subtask
	if saved.ParentTaskId != "" {
		evt := model.NewWebSocketEvent(model.WebsocketEventAgentDelegation, "", saved.ChannelId, "", nil, "")
		evt.Add("task_id", saved.Id)
		evt.Add("parent_task_id", saved.ParentTaskId)
		evt.Add("root_task_id", saved.RootTaskId)
		evt.Add("agent_id", saved.AgentId)
		s.dispatcher.publishEvent(evt)
	}

	// Enqueue for execution (async — returns immediately)
	s.dispatcher.Enqueue(saved)

	return saved, nil
}

// GetTask returns the current state of a task.
func (s *AgentRuntimeService) GetTask(_ request.CTX, taskId string) (*model.AgentTask, error) {
	return s.store.Agent().GetAgentTask(taskId)
}

// GetTaskTree returns the full delegation DAG for a root task.
func (s *AgentRuntimeService) GetTaskTree(_ request.CTX, rootTaskId string) ([]*model.AgentTask, error) {
	return s.store.Agent().GetTaskTree(rootTaskId)
}

// GetTaskEvents returns observability events for a task.
func (s *AgentRuntimeService) GetTaskEvents(_ request.CTX, taskId string, page, perPage int) ([]*model.AgentTaskEvent, error) {
	return s.store.Agent().GetAgentTaskEvents(taskId, page, perPage)
}

// GetActiveTasksForAgent returns all non-terminal tasks for an agent.
func (s *AgentRuntimeService) GetActiveTasksForAgent(_ request.CTX, agentId string) ([]*model.AgentTask, error) {
	return s.store.Agent().GetActiveTasksForAgent(agentId)
}

// GetMetrics returns a snapshot of in-memory per-agent runtime metrics.
func (s *AgentRuntimeService) GetMetrics() []AgentMetrics {
	return s.obs.GetMetrics()
}
