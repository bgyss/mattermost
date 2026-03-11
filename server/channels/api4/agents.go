// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (api *API) InitAgents() {
	// Legacy bridge routes (keep for backwards compat)
	api.BaseRoutes.Agents.Handle("", api.APISessionRequired(getAgents)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("/status", api.APISessionRequired(getAgentsStatus)).Methods(http.MethodGet)
	api.BaseRoutes.LLMServices.Handle("", api.APISessionRequired(getLLMServices)).Methods(http.MethodGet)

	// Agent Definition CRUD
	api.BaseRoutes.Agents.Handle("/definitions", api.APISessionRequired(listAgentDefinitions)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("", api.APISessionRequired(createAgentDefinition)).Methods(http.MethodPost)
	api.BaseRoutes.Agents.Handle("/{agent_id:[A-Za-z0-9]+}", api.APISessionRequired(patchAgentDefinition)).Methods(http.MethodPatch)
	api.BaseRoutes.Agents.Handle("/{agent_id:[A-Za-z0-9]+}", api.APISessionRequired(deleteAgentDefinition)).Methods(http.MethodDelete)

	// Task submission and status
	api.BaseRoutes.Agents.Handle("/tasks", api.APISessionRequired(submitAgentTask)).Methods(http.MethodPost)
	api.BaseRoutes.Agents.Handle("/tasks/{task_id:[A-Za-z0-9]+}", api.APISessionRequired(getAgentTask)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("/tasks/{task_id:[A-Za-z0-9]+}/tree", api.APISessionRequired(getAgentTaskTree)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("/tasks/{task_id:[A-Za-z0-9]+}/events", api.APISessionRequired(getAgentTaskEvents)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("/{agent_id:[A-Za-z0-9]+}/tasks/active", api.APISessionRequired(getActiveAgentTasks)).Methods(http.MethodGet)

	// Memory
	api.BaseRoutes.Agents.Handle("/{agent_id:[A-Za-z0-9]+}/memory", api.APISessionRequired(getAgentMemory)).Methods(http.MethodGet)
	api.BaseRoutes.Agents.Handle("/{agent_id:[A-Za-z0-9]+}/memory", api.APISessionRequired(clearAgentMemory)).Methods(http.MethodDelete)

	// Metrics
	api.BaseRoutes.Agents.Handle("/metrics", api.APISessionRequired(getAgentMetrics)).Methods(http.MethodGet)

	// Coordination channels (observable agent-to-agent channels)
	api.BaseRoutes.Agents.Handle("/coordination-channels", api.APISessionRequired(getAgentCoordinationChannels)).Methods(http.MethodGet)

	// Demo setup
	api.BaseRoutes.Agents.Handle("/demo-setup", api.APISessionRequired(postDemoSetup)).Methods(http.MethodPost)
}

// ---------------------------------------------------------------------------
// Legacy bridge handlers (kept for backwards compat with mattermost-plugin-agents)
// ---------------------------------------------------------------------------

func getAgentsStatus(c *Context, w http.ResponseWriter, r *http.Request) {
	available, reason := c.App.GetAIPluginBridgeStatus(c.AppContext)

	resp := &model.AgentsIntegrityResponse{
		Available: available,
		Reason:    reason,
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgentsStatus", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(jsonData); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getAgents(c *Context, w http.ResponseWriter, r *http.Request) {
	agents, appErr := c.App.GetAgents(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.getAgents", "app.agents.get_agents.app_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
		return
	}

	jsonData, err := json.Marshal(agents)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgents", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(jsonData); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getLLMServices(c *Context, w http.ResponseWriter, r *http.Request) {
	services, appErr := c.App.GetLLMServices(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = model.NewAppError("Api4.getLLMServices", "app.agents.get_services.app_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
		return
	}

	jsonData, err := json.Marshal(services)
	if err != nil {
		c.Err = model.NewAppError("Api4.getLLMServices", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(jsonData); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// ---------------------------------------------------------------------------
// AgentDefinition handlers
// ---------------------------------------------------------------------------

func listAgentDefinitions(c *Context, w http.ResponseWriter, r *http.Request) {
	defs, appErr := c.App.ListAllAgentDefinitions(c.AppContext)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if err := json.NewEncoder(w).Encode(defs); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func createAgentDefinition(c *Context, w http.ResponseWriter, r *http.Request) {
	var def model.AgentDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		c.Err = model.NewAppError("Api4.createAgentDefinition", "api.invalid_body", nil, err.Error(), http.StatusBadRequest)
		return
	}
	def.OwnerUserId = c.AppContext.Session().UserId

	created, appErr := c.App.CreateAgentDefinition(c.AppContext, &def)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(created); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func patchAgentDefinition(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAgentId()
	if c.Err != nil {
		return
	}

	var patch model.PatchAgentDefinition
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		c.Err = model.NewAppError("Api4.patchAgentDefinition", "api.invalid_body", nil, err.Error(), http.StatusBadRequest)
		return
	}

	updated, appErr := c.App.PatchAgentDefinition(c.AppContext, c.Params.AgentId, &patch)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(updated); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func deleteAgentDefinition(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAgentId()
	if c.Err != nil {
		return
	}

	if appErr := c.App.DeleteAgentDefinition(c.AppContext, c.Params.AgentId); appErr != nil {
		c.Err = appErr
		return
	}

	ReturnStatusOK(w)
}

// ---------------------------------------------------------------------------
// Task handlers
// ---------------------------------------------------------------------------

func submitAgentTask(c *Context, w http.ResponseWriter, r *http.Request) {
	var req model.SubmitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.Err = model.NewAppError("Api4.submitAgentTask", "api.invalid_body", nil, err.Error(), http.StatusBadRequest)
		return
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.submitAgentTask", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	task, err := svc.SubmitTask(c.AppContext, &req)
	if err != nil {
		c.Err = model.NewAppError("Api4.submitAgentTask", "app.agent_task.submit.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getAgentTask(c *Context, w http.ResponseWriter, r *http.Request) {
	taskId := c.Params.TaskId
	if taskId == "" {
		c.SetInvalidParam("task_id")
		return
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.getAgentTask", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	task, err := svc.GetTask(c.AppContext, taskId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgentTask", "app.agent_task.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(task); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getAgentTaskTree(c *Context, w http.ResponseWriter, r *http.Request) {
	taskId := c.Params.TaskId
	if taskId == "" {
		c.SetInvalidParam("task_id")
		return
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.getAgentTaskTree", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	tree, err := svc.GetTaskTree(c.AppContext, taskId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgentTaskTree", "app.agent_task.get_tree.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(tree); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getAgentTaskEvents(c *Context, w http.ResponseWriter, r *http.Request) {
	taskId := c.Params.TaskId
	if taskId == "" {
		c.SetInvalidParam("task_id")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage := 50
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 && n <= 200 {
			perPage = n
		}
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.getAgentTaskEvents", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	events, err := svc.GetTaskEvents(c.AppContext, taskId, page, perPage)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgentTaskEvents", "app.agent_task.get_events.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(events); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getActiveAgentTasks(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAgentId()
	if c.Err != nil {
		return
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.getActiveAgentTasks", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	tasks, err := svc.GetActiveTasksForAgent(c.AppContext, c.Params.AgentId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getActiveAgentTasks", "app.agent_task.list_active.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// ---------------------------------------------------------------------------
// Memory handlers
// ---------------------------------------------------------------------------

func getAgentMemory(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAgentId()
	if c.Err != nil {
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = model.AgentMemoryScopeGlobal
	}
	scopeId := r.URL.Query().Get("scope_id")

	mems, err := c.App.Srv().Store().Agent().GetAgentMemoryByScope(c.Params.AgentId, scope, scopeId)
	if err != nil {
		c.Err = model.NewAppError("Api4.getAgentMemory", "app.agent_memory.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(mems); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func clearAgentMemory(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireAgentId()
	if c.Err != nil {
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = model.AgentMemoryScopeGlobal
	}
	scopeId := r.URL.Query().Get("scope_id")

	if err := c.App.Srv().Store().Agent().DeleteAgentMemory(c.Params.AgentId, scope, scopeId); err != nil {
		c.Err = model.NewAppError("Api4.clearAgentMemory", "app.agent_memory.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
		return
	}

	ReturnStatusOK(w)
}

// ---------------------------------------------------------------------------
// Metrics handler
// ---------------------------------------------------------------------------

func getAgentMetrics(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	svc := c.App.AgentRuntimeService()
	if svc == nil {
		c.Err = model.NewAppError("Api4.getAgentMetrics", "api.agent_runtime_not_available", nil, "", http.StatusServiceUnavailable)
		return
	}

	if err := json.NewEncoder(w).Encode(svc.GetMetrics()); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// ---------------------------------------------------------------------------
// Coordination channels handler
// ---------------------------------------------------------------------------

func getAgentCoordinationChannels(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	teamId := r.URL.Query().Get("team_id")
	if teamId == "" {
		c.SetInvalidParam("team_id")
		return
	}

	channels, appErr := c.App.GetAgentCoordinationChannels(c.AppContext, teamId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(channels); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// ---------------------------------------------------------------------------
// Demo setup handler
// ---------------------------------------------------------------------------

func postDemoSetup(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	var req model.DemoSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.Err = model.NewAppError("Api4.postDemoSetup", "api.invalid_body", nil, err.Error(), http.StatusBadRequest)
		return
	}

	result, appErr := c.App.ProvisionDemoSetup(c.AppContext, &req)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
