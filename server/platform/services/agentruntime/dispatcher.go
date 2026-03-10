// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const (
	// dispatcherPollInterval is how often the dispatcher polls the DB for pending tasks
	// (crash-recovery path — in-memory channel is the fast path).
	dispatcherPollInterval = 10 * time.Second

	// defaultMaxConcurrency is the per-agent goroutine cap when not specified.
	defaultMaxConcurrency = 5
)

// workItem is an in-memory queued task + its resolved definition + LLM service.
type workItem struct {
	task *model.AgentTask
	def  *model.AgentDefinition
	llm  LLMService
}

// semaphoreMap tracks per-agent concurrency semaphores.
type semaphoreMap struct {
	mu   sync.Mutex
	sems map[string]chan struct{} // agentId → buffered channel (semaphore)
}

func newSemaphoreMap() *semaphoreMap {
	return &semaphoreMap{sems: make(map[string]chan struct{})}
}

func (m *semaphoreMap) acquire(agentId string, max int) {
	m.mu.Lock()
	sem, ok := m.sems[agentId]
	if !ok {
		if max <= 0 {
			max = defaultMaxConcurrency
		}
		sem = make(chan struct{}, max)
		m.sems[agentId] = sem
	}
	m.mu.Unlock()
	sem <- struct{}{} // blocks until a slot is free
}

func (m *semaphoreMap) release(agentId string) {
	m.mu.Lock()
	sem := m.sems[agentId]
	m.mu.Unlock()
	if sem != nil {
		<-sem
	}
}

// dispatcher manages the in-memory work queue and dispatches tasks to the executor.
type dispatcher struct {
	store    store.Store
	app      AgentAppIface
	srvId    string
	log      *mlog.Logger
	exec     *executor
	llmCache map[string]LLMService // agentId → LLMService (cached per agent)
	llmMu    sync.RWMutex

	workCh chan workItem
	sems   *semaphoreMap
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func newDispatcher(st store.Store, app AgentAppIface, srvId string, log *mlog.Logger, toolReg *ToolRegistry, memMgr *MemoryManager, obs *observabilityService) *dispatcher {
	exec := newExecutor(st, app, log, toolReg, memMgr, obs)
	return &dispatcher{
		store:    st,
		app:      app,
		srvId:    srvId,
		log:      log,
		exec:     exec,
		llmCache: make(map[string]LLMService),
		workCh:   make(chan workItem, 256),
		sems:     newSemaphoreMap(),
		stopCh:   make(chan struct{}),
	}
}

// Start launches the background goroutines.
func (d *dispatcher) Start() {
	// Worker goroutine that processes items from workCh
	d.wg.Add(1)
	go d.runWorker()

	// Periodic DB poll for crash-recovery (tasks left in pending state by crashed servers)
	d.wg.Add(1)
	go d.runDBPoller()
}

// Stop signals workers to stop and waits for them to finish.
func (d *dispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

// Enqueue adds a task to the in-memory work queue.
// Returns immediately; execution is async.
func (d *dispatcher) Enqueue(task *model.AgentTask) {
	// Resolve agent definition and LLM synchronously so we can start ASAP.
	// If this fails we fall through to the DB poller as a recovery path.
	def, llm, err := d.resolveAgent(task.AgentId)
	if err != nil {
		d.log.Warn("dispatcher: could not resolve agent, task will be retried by poller",
			mlog.String("agent_id", task.AgentId),
			mlog.Err(err),
		)
		return
	}

	select {
	case d.workCh <- workItem{task: task, def: def, llm: llm}:
	case <-d.stopCh:
	default:
		d.log.Warn("dispatcher: work channel full, task will be retried by poller",
			mlog.String("task_id", task.Id))
	}
}

// runWorker consumes from workCh and executes tasks respecting per-agent concurrency.
func (d *dispatcher) runWorker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.stopCh:
			return
		case item := <-d.workCh:
			// Claim in DB before starting (cluster-safe)
			claimed, claimErr := d.store.Agent().ClaimPendingTask(item.task.AgentId, d.srvId)
			if claimErr != nil || claimed == nil {
				// Already claimed by another node or gone
				continue
			}

			max := item.def.MaxConcurrency
			if max <= 0 {
				max = defaultMaxConcurrency
			}
			d.sems.acquire(item.task.AgentId, max)

			d.wg.Add(1)
			go func(wi workItem, claimed *model.AgentTask) {
				defer d.wg.Done()
				defer d.sems.release(claimed.AgentId)
				d.exec.Execute(claimed, wi.def, wi.llm)
			}(item, claimed)
		}
	}
}

// runDBPoller periodically scans for pending tasks that weren't picked up
// (e.g. because this server restarted mid-flight or a channel was full).
func (d *dispatcher) runDBPoller() {
	defer d.wg.Done()
	ticker := time.NewTicker(dispatcherPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.pollPendingTasks()
		}
	}
}

func (d *dispatcher) pollPendingTasks() {
	// Get all agent definitions to know which agents this service manages
	// We attempt to claim pending tasks for any known agent.
	// This is intentionally simple — in a large cluster, only a subset of agents
	// will have pending tasks at any given time.
	claimed, err := d.store.Agent().ClaimPendingTask("", d.srvId)
	if err != nil || claimed == nil {
		return
	}

	def, llm, resolveErr := d.resolveAgent(claimed.AgentId)
	if resolveErr != nil {
		d.log.Warn("dispatcher: poller could not resolve agent",
			mlog.String("agent_id", claimed.AgentId),
			mlog.Err(resolveErr),
		)
		return
	}

	max := def.MaxConcurrency
	if max <= 0 {
		max = defaultMaxConcurrency
	}
	d.sems.acquire(claimed.AgentId, max)

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer d.sems.release(claimed.AgentId)
		d.exec.Execute(claimed, def, llm)
	}()
}

// resolveAgent looks up the agent definition and returns a cached (or newly built) LLM client.
func (d *dispatcher) resolveAgent(agentId string) (*model.AgentDefinition, LLMService, error) {
	def, err := d.store.Agent().GetAgentDefinition(agentId)
	if err != nil {
		return nil, nil, err
	}

	d.llmMu.RLock()
	llm, cached := d.llmCache[agentId]
	d.llmMu.RUnlock()

	if !cached {
		cfg := d.llmConfigForDef(def)
		newLLM, buildErr := NewLLMService(cfg)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		d.llmMu.Lock()
		d.llmCache[agentId] = newLLM
		d.llmMu.Unlock()
		llm = newLLM
	}

	return def, llm, nil
}

// llmConfigForDef derives an LLMServiceConfig from an AgentDefinition.
//
// Provider resolution order:
//  1. Explicit LLMServiceId field on the definition ("openclaw", "anthropic", "openai")
//  2. ModelId prefix ("gpt-*" → openai, "claude-*" → anthropic)
//  3. Default: "openclaw" (local OpenClaw gateway)
//
// API keys / tokens come from environment variables. In Phase 6+ these will be
// stored in a proper LLMService configuration table.
func (d *dispatcher) llmConfigForDef(def *model.AgentDefinition) LLMServiceConfig {
	provider := resolveProvider(def.LLMServiceId, def.ModelId)

	apiKey := ""
	switch provider {
	case "openai":
		apiKey = getEnvOrDefault("MM_AGENTS_OPENAI_API_KEY", "")
	case "anthropic":
		apiKey = getEnvOrDefault("MM_AGENTS_ANTHROPIC_API_KEY", "")
	default: // openclaw
		// Accept either the Mattermost-specific var or the native OpenClaw var
		apiKey = getEnvOrDefault("MM_AGENTS_OPENCLAW_TOKEN",
			getEnvOrDefault("OPENCLAW_GATEWAY_TOKEN", ""))
	}

	return LLMServiceConfig{
		Provider:        provider,
		APIKey:          apiKey,
		BaseURL:         getEnvOrDefault("MM_AGENTS_LLM_BASE_URL", ""),
		ExternalAgentId: def.Id, // each AgentDefinition maps to its own OpenClaw agent
	}
}

// publishEvent forwards a WebSocket event to connected clients via the app layer.
func (d *dispatcher) publishEvent(evt *model.WebSocketEvent) {
	d.app.Publish(evt)
}

// resolveProvider picks a provider string from an explicit service ID or model name prefix.
func resolveProvider(serviceId, modelId string) string {
	if serviceId != "" {
		return strings.ToLower(serviceId)
	}
	if strings.HasPrefix(modelId, "gpt") || strings.HasPrefix(modelId, "o1") || strings.HasPrefix(modelId, "o3") {
		return "openai"
	}
	if strings.HasPrefix(modelId, "claude") {
		return "anthropic"
	}
	return "openclaw"
}
