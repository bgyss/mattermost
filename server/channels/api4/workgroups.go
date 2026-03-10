// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (api *API) InitWorkgroups() {
	// GET  /api/v4/workgroups                  - list workgroups
	api.BaseRoutes.Workgroups.Handle("", api.APISessionRequired(listWorkgroups)).Methods(http.MethodGet)
	// POST /api/v4/workgroups                  - provision from template
	api.BaseRoutes.Workgroups.Handle("", api.APISessionRequired(provisionWorkgroup)).Methods(http.MethodPost)
	// GET  /api/v4/workgroups/templates        - list available templates
	api.BaseRoutes.Workgroups.Handle("/templates", api.APISessionRequired(listWorkgroupTemplates)).Methods(http.MethodGet)
	// GET  /api/v4/workgroups/{id}             - get workgroup
	api.BaseRoutes.Workgroup.Handle("", api.APISessionRequired(getWorkgroup)).Methods(http.MethodGet)
	// DELETE /api/v4/workgroups/{id}           - deprovision workgroup
	api.BaseRoutes.Workgroup.Handle("", api.APISessionRequired(deleteWorkgroup)).Methods(http.MethodDelete)
	// GET  /api/v4/workgroups/{id}/agents      - list agents in workgroup
	api.BaseRoutes.Workgroup.Handle("/agents", api.APISessionRequired(listWorkgroupAgents)).Methods(http.MethodGet)
}

func listWorkgroups(c *Context, w http.ResponseWriter, r *http.Request) {
	teamId := r.URL.Query().Get("team_id")
	if teamId == "" {
		c.SetInvalidParam("team_id")
		return
	}

	workgroups, appErr := c.App.GetWorkgroupsForTeam(c.AppContext, teamId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(workgroups); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func provisionWorkgroup(c *Context, w http.ResponseWriter, r *http.Request) {
	// Require system admin for provisioning
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	var req model.ProvisionWorkgroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.Err = model.NewAppError("Api4.provisionWorkgroup", "api.invalid_body", nil, err.Error(), http.StatusBadRequest)
		return
	}

	workgroup, appErr := c.App.ProvisionWorkgroup(c.AppContext, &req)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(workgroup); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func listWorkgroupTemplates(c *Context, w http.ResponseWriter, r *http.Request) {
	templates := c.App.GetWorkgroupTemplates()
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getWorkgroup(c *Context, w http.ResponseWriter, r *http.Request) {
	workgroupId := c.Params.WorkgroupId
	if workgroupId == "" {
		c.SetInvalidParam("workgroup_id")
		return
	}

	workgroup, appErr := c.App.GetWorkgroup(c.AppContext, workgroupId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(workgroup); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func deleteWorkgroup(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	workgroupId := c.Params.WorkgroupId
	if workgroupId == "" {
		c.SetInvalidParam("workgroup_id")
		return
	}

	if appErr := c.App.DeleteWorkgroup(c.AppContext, workgroupId); appErr != nil {
		c.Err = appErr
		return
	}

	ReturnStatusOK(w)
}

func listWorkgroupAgents(c *Context, w http.ResponseWriter, r *http.Request) {
	workgroupId := c.Params.WorkgroupId
	if workgroupId == "" {
		c.SetInvalidParam("workgroup_id")
		return
	}

	defs, appErr := c.App.GetAgentDefinitionsByWorkgroup(c.AppContext, workgroupId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(defs); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
