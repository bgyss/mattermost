// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import "encoding/json"

// Workgroup types
const (
	WorkgroupTypeDepartment = "department"
	WorkgroupTypeExecutive  = "executive"
)

// Workgroup statuses
const (
	WorkgroupStatusProvisioning = "provisioning"
	WorkgroupStatusActive       = "active"
	WorkgroupStatusSuspended    = "suspended"
)

// LLM tiers
const (
	LLMTierStandard = "standard"
	LLMTierPremium  = "premium"
)

// WorkgroupChannelIDs holds the channels provisioned for each workgroup.
type WorkgroupChannelIDs struct {
	General  string `json:"general"`
	Internal string `json:"internal"`
	Reports  string `json:"reports"`
	// CoordinationChannels maps peer workgroup ID → the ChannelTypeAgentDirect channel ID
	// shared between this workgroup's head agent and that peer's head agent.
	// This field is populated lazily; existing rows without it simply have a nil map.
	CoordinationChannels map[string]string `json:"coordination_channels,omitempty"`
}

// SetCoordinationChannel records or overwrites the coordination channel with a peer workgroup.
func (c *WorkgroupChannelIDs) SetCoordinationChannel(peerWorkgroupId, channelId string) {
	if c.CoordinationChannels == nil {
		c.CoordinationChannels = make(map[string]string)
	}
	c.CoordinationChannels[peerWorkgroupId] = channelId
}

// GetCoordinationChannel returns the coordination channel ID for a peer workgroup, or "".
func (c *WorkgroupChannelIDs) GetCoordinationChannel(peerWorkgroupId string) string {
	if c.CoordinationChannels == nil {
		return ""
	}
	return c.CoordinationChannels[peerWorkgroupId]
}

// Workgroup represents a department or executive office in the virtual company.
type Workgroup struct {
	Id          string              `json:"id"`
	TeamId      string              `json:"team_id"`
	Name        string              `json:"name"`         // e.g. "marketing"
	DisplayName string              `json:"display_name"` // e.g. "Marketing"
	Type        string              `json:"type"`         // department|executive
	HeadAgentId string              `json:"head_agent_id"`
	Status      string              `json:"status"` // provisioning|active|suspended
	ChannelIds  WorkgroupChannelIDs `json:"channel_ids"`
	CreateAt    int64               `json:"create_at"`
	UpdateAt    int64               `json:"update_at"`
	DeleteAt    int64               `json:"delete_at"`
}

func (w *Workgroup) IsValid() *AppError {
	if w.Id == "" {
		return NewAppError("Workgroup.IsValid", "model.workgroup.is_valid.id.app_error", nil, "", 400)
	}
	if w.Name == "" {
		return NewAppError("Workgroup.IsValid", "model.workgroup.is_valid.name.app_error", nil, "", 400)
	}
	if w.Type != WorkgroupTypeDepartment && w.Type != WorkgroupTypeExecutive {
		return NewAppError("Workgroup.IsValid", "model.workgroup.is_valid.type.app_error", nil, "", 400)
	}
	return nil
}

func (w *Workgroup) PreSave() {
	if w.Id == "" {
		w.Id = NewId()
	}
	now := GetMillis()
	w.CreateAt = now
	w.UpdateAt = now
}

func (w *Workgroup) PreUpdate() {
	w.UpdateAt = GetMillis()
}

// AgentSpec describes one agent within a WorkgroupTemplate.
type AgentSpec struct {
	Role         string   `json:"role"` // head|specialist|executive
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Capabilities []string `json:"capabilities"`
	Tools        []string `json:"tools"`
}

// WorkgroupTemplate is a built-in preset for provisioning a workgroup.
type WorkgroupTemplate struct {
	Id              string      `json:"id"`
	Name            string      `json:"name"`         // e.g. "marketing"
	DisplayName     string      `json:"display_name"` // e.g. "Marketing"
	AgentSpecs      []AgentSpec `json:"agent_specs"`
	DefaultLLMTier  string      `json:"default_llm_tier"`
}

// ProvisionWorkgroupRequest is the request body for POST /api/v4/workgroups.
type ProvisionWorkgroupRequest struct {
	TemplateId  string      `json:"template_id"`
	TeamId      string      `json:"team_id"`
	Name        string      `json:"name,omitempty"`
	DisplayName string      `json:"display_name,omitempty"`
	AgentSpecs  []AgentSpec `json:"agent_specs,omitempty"` // overrides template defaults
}

func (r *ProvisionWorkgroupRequest) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// DemoSetupRequest is the body for POST /api/v4/agents/demo-setup.
type DemoSetupRequest struct {
	TeamId string `json:"team_id"`
	// ObserverUserIds lists users who will be added to all agent coordination channels
	// for full visibility. Typically the admin running the demo.
	ObserverUserIds []string `json:"observer_user_ids,omitempty"`
	// LLMServiceId overrides the LLM backend for all demo agents (e.g. "openclaw", "anthropic").
	LLMServiceId string `json:"llm_service_id,omitempty"`
}

// DemoSetupResponse summarises what was created.
type DemoSetupResponse struct {
	ExecutiveWorkgroup       *Workgroup `json:"executive_workgroup"`
	SalesWorkgroup           *Workgroup `json:"sales_workgroup"`
	CoordinationChannelId    string     `json:"coordination_channel_id"`
	ExecutiveAgentId         string     `json:"executive_agent_id"`
	SalesHeadAgentId         string     `json:"sales_head_agent_id"`
	SalesSpecialistAgentIds  []string   `json:"sales_specialist_agent_ids"`
}
