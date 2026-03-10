// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// AgentDefinition roles
const (
	AgentRoleHead       = "head"
	AgentRoleSpecialist = "specialist"
	AgentRoleExecutive  = "executive"
)

// ModelParameters holds per-agent LLM hyperparameters.
type ModelParameters struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
}

// MemoryConfig holds agent memory configuration.
type MemoryConfig struct {
	Enabled         bool `json:"enabled"`
	MaxGlobalKeys   int  `json:"max_global_keys"`
	SemanticTopK    int  `json:"semantic_top_k"`
	ExpireDays      int  `json:"expire_days"`
}

// AgentDefinition is the specification of an agent — its identity, LLM config, tools, and memory settings.
type AgentDefinition struct {
	Id              string          `json:"id"`
	WorkgroupId     string          `json:"workgroup_id"`   // nullable: empty for standalone agents
	Role            string          `json:"role"`           // head|specialist|executive
	BotUserId       string          `json:"bot_user_id"`
	DisplayName     string          `json:"display_name"`
	SystemPrompt    string          `json:"system_prompt"`
	LLMServiceId    string          `json:"llm_service_id"` // which LLM provider config to use
	ModelId         string          `json:"model_id"`       // e.g. "claude-sonnet-4-6"
	ModelParameters ModelParameters `json:"model_parameters"`
	Tools           []string        `json:"tools"`
	Capabilities    []string        `json:"capabilities"`
	MaxConcurrency  int             `json:"max_concurrency"`
	PoolSize        int             `json:"pool_size"`
	MemoryConfig    MemoryConfig    `json:"memory_config"`
	OwnerUserId     string          `json:"owner_user_id"`
	CreateAt        int64           `json:"create_at"`
	UpdateAt        int64           `json:"update_at"`
	DeleteAt        int64           `json:"delete_at"`
}

func (a *AgentDefinition) IsValid() *AppError {
	if a.Id == "" {
		return NewAppError("AgentDefinition.IsValid", "model.agent_definition.is_valid.id.app_error", nil, "", 400)
	}
	if a.DisplayName == "" {
		return NewAppError("AgentDefinition.IsValid", "model.agent_definition.is_valid.display_name.app_error", nil, "", 400)
	}
	if a.Role != AgentRoleHead && a.Role != AgentRoleSpecialist && a.Role != AgentRoleExecutive {
		return NewAppError("AgentDefinition.IsValid", "model.agent_definition.is_valid.role.app_error", nil, "", 400)
	}
	return nil
}

func (a *AgentDefinition) PreSave() {
	if a.Id == "" {
		a.Id = NewId()
	}
	now := GetMillis()
	a.CreateAt = now
	a.UpdateAt = now
	if a.MaxConcurrency == 0 {
		a.MaxConcurrency = 5
	}
	if a.PoolSize == 0 {
		a.PoolSize = 1
	}
	// Default provider: OpenClaw local gateway.
	// Override by setting LLMServiceId to "anthropic" or "openai".
	if a.LLMServiceId == "" {
		a.LLMServiceId = "openclaw"
	}
}

func (a *AgentDefinition) PreUpdate() {
	a.UpdateAt = GetMillis()
}

// PatchAgentDefinition holds optional update fields.
type PatchAgentDefinition struct {
	DisplayName     *string          `json:"display_name,omitempty"`
	SystemPrompt    *string          `json:"system_prompt,omitempty"`
	LLMServiceId    *string          `json:"llm_service_id,omitempty"`
	ModelId         *string          `json:"model_id,omitempty"`
	ModelParameters *ModelParameters `json:"model_parameters,omitempty"`
	Tools           *[]string        `json:"tools,omitempty"`
	Capabilities    *[]string        `json:"capabilities,omitempty"`
	MaxConcurrency  *int             `json:"max_concurrency,omitempty"`
}

func (a *AgentDefinition) Patch(patch *PatchAgentDefinition) {
	if patch.DisplayName != nil {
		a.DisplayName = *patch.DisplayName
	}
	if patch.SystemPrompt != nil {
		a.SystemPrompt = *patch.SystemPrompt
	}
	if patch.LLMServiceId != nil {
		a.LLMServiceId = *patch.LLMServiceId
	}
	if patch.ModelId != nil {
		a.ModelId = *patch.ModelId
	}
	if patch.ModelParameters != nil {
		a.ModelParameters = *patch.ModelParameters
	}
	if patch.Tools != nil {
		a.Tools = *patch.Tools
	}
	if patch.Capabilities != nil {
		a.Capabilities = *patch.Capabilities
	}
	if patch.MaxConcurrency != nil {
		a.MaxConcurrency = *patch.MaxConcurrency
	}
}
