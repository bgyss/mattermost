// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// defaultHeadModelID is the LLM model used for head and executive agents.
const defaultHeadModelID = "claude-opus-4-6"

// defaultSpecialistModelID is the LLM model used for specialist agents.
const defaultSpecialistModelID = "claude-sonnet-4-6"

// ---------------------------------------------------------------------------
// Built-in WorkgroupTemplates
// ---------------------------------------------------------------------------

// builtinTemplates contains pre-defined department configurations.
var builtinTemplates = []model.WorkgroupTemplate{
	{
		Id:             "executive",
		Name:           "executive",
		DisplayName:    "Executive Office",
		DefaultLLMTier: model.LLMTierPremium,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleExecutive,
				Name:         "CEO",
				SystemPrompt: executiveSystemPrompt,
				Capabilities: []string{"strategy", "coordination", "delegation"},
				Tools:        []string{"search_channels", "create_post", "delegate_task"},
			},
		},
	},
	{
		Id:             "marketing",
		Name:           "marketing",
		DisplayName:    "Marketing",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Marketing Head",
				SystemPrompt: marketingHeadSystemPrompt,
				Capabilities: []string{"marketing", "campaigns", "branding"},
				Tools:        []string{"search_channels", "create_post", "delegate_task", "route_to_capability"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Campaign Strategist",
				SystemPrompt: campaignStrategistSystemPrompt,
				Capabilities: []string{"campaign_strategy", "market_research"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Content Writer",
				SystemPrompt: contentWriterSystemPrompt,
				Capabilities: []string{"content_writing", "copywriting", "social_media"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "engineering",
		Name:           "engineering",
		DisplayName:    "Engineering",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Engineering Head",
				SystemPrompt: engineeringHeadSystemPrompt,
				Capabilities: []string{"engineering", "architecture", "planning"},
				Tools:        []string{"search_channels", "create_post", "delegate_task", "route_to_capability"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Backend Engineer",
				SystemPrompt: backendEngineerSystemPrompt,
				Capabilities: []string{"backend", "api", "database"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Frontend Engineer",
				SystemPrompt: frontendEngineerSystemPrompt,
				Capabilities: []string{"frontend", "ui", "react"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "sales",
		Name:           "sales",
		DisplayName:    "Sales",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Sales Head",
				SystemPrompt: salesHeadSystemPrompt,
				Capabilities: []string{"sales", "pipeline", "crm"},
				Tools:        []string{"search_channels", "create_post", "delegate_task", "route_to_capability"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Account Executive",
				SystemPrompt: accountExecutiveSystemPrompt,
				Capabilities: []string{"account_management", "negotiation"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Sales Development Rep",
				SystemPrompt: sdrSystemPrompt,
				Capabilities: []string{"lead_generation", "outreach", "qualification"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "finance",
		Name:           "finance",
		DisplayName:    "Finance",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Finance Head",
				SystemPrompt: financeHeadSystemPrompt,
				Capabilities: []string{"finance", "budgeting", "reporting"},
				Tools:        []string{"search_channels", "create_post", "delegate_task"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Financial Analyst",
				SystemPrompt: financialAnalystSystemPrompt,
				Capabilities: []string{"financial_analysis", "forecasting", "modeling"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Accountant",
				SystemPrompt: accountantSystemPrompt,
				Capabilities: []string{"accounting", "bookkeeping", "compliance"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "research",
		Name:           "research",
		DisplayName:    "Research & Development",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Research Head",
				SystemPrompt: researchHeadSystemPrompt,
				Capabilities: []string{"research", "analysis", "synthesis"},
				Tools:        []string{"search_channels", "create_post", "delegate_task", "route_to_capability"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Research Analyst",
				SystemPrompt: researchAnalystSystemPrompt,
				Capabilities: []string{"data_analysis", "literature_review"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Technical Researcher",
				SystemPrompt: technicalResearcherSystemPrompt,
				Capabilities: []string{"technical_research", "prototyping"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "product",
		Name:           "product",
		DisplayName:    "Product",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Product Head",
				SystemPrompt: productHeadSystemPrompt,
				Capabilities: []string{"product_management", "roadmap", "prioritization"},
				Tools:        []string{"search_channels", "create_post", "delegate_task"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Product Manager",
				SystemPrompt: productManagerSystemPrompt,
				Capabilities: []string{"requirements", "user_stories", "feature_specs"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "UX Researcher",
				SystemPrompt: uxResearcherSystemPrompt,
				Capabilities: []string{"ux_research", "user_interviews", "usability"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "manufacturing",
		Name:           "manufacturing",
		DisplayName:    "Manufacturing",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Manufacturing Head",
				SystemPrompt: manufacturingHeadSystemPrompt,
				Capabilities: []string{"manufacturing", "production", "quality"},
				Tools:        []string{"search_channels", "create_post", "delegate_task"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Production Planner",
				SystemPrompt: productionPlannerSystemPrompt,
				Capabilities: []string{"production_planning", "scheduling"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Quality Engineer",
				SystemPrompt: qualityEngineerSystemPrompt,
				Capabilities: []string{"quality_control", "inspection", "compliance"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
	{
		Id:             "supply_chain",
		Name:           "supply_chain",
		DisplayName:    "Supply Chain",
		DefaultLLMTier: model.LLMTierStandard,
		AgentSpecs: []model.AgentSpec{
			{
				Role:         model.AgentRoleHead,
				Name:         "Supply Chain Head",
				SystemPrompt: supplyChainHeadSystemPrompt,
				Capabilities: []string{"supply_chain", "procurement", "logistics"},
				Tools:        []string{"search_channels", "create_post", "delegate_task"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Procurement Specialist",
				SystemPrompt: procurementSpecialistSystemPrompt,
				Capabilities: []string{"procurement", "vendor_management"},
				Tools:        []string{"search_channels", "create_post"},
			},
			{
				Role:         model.AgentRoleSpecialist,
				Name:         "Logistics Coordinator",
				SystemPrompt: logisticsCoordinatorSystemPrompt,
				Capabilities: []string{"logistics", "shipping", "inventory"},
				Tools:        []string{"search_channels", "create_post"},
			},
		},
	},
}

// ---------------------------------------------------------------------------
// Template access
// ---------------------------------------------------------------------------

// GetWorkgroupTemplates returns all built-in workgroup templates.
func (a *App) GetWorkgroupTemplates() []model.WorkgroupTemplate {
	return builtinTemplates
}

// GetWorkgroupTemplate returns a single template by ID.
func (a *App) GetWorkgroupTemplate(id string) (*model.WorkgroupTemplate, *model.AppError) {
	for _, t := range builtinTemplates {
		if t.Id == id {
			copy := t
			return &copy, nil
		}
	}
	return nil, model.NewAppError("GetWorkgroupTemplate", "app.workgroup.template_not_found", nil, "", http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// Workgroup CRUD
// ---------------------------------------------------------------------------

// GetWorkgroupsForTeam returns all active workgroups for a team.
func (a *App) GetWorkgroupsForTeam(rctx request.CTX, teamId string) ([]*model.Workgroup, *model.AppError) {
	wgs, err := a.Srv().Store().Agent().GetWorkgroupsForTeam(teamId)
	if err != nil {
		return nil, model.NewAppError("GetWorkgroupsForTeam", "app.workgroup.get_workgroups.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return wgs, nil
}

// GetWorkgroup returns a workgroup by ID.
func (a *App) GetWorkgroup(rctx request.CTX, id string) (*model.Workgroup, *model.AppError) {
	wg, err := a.Srv().Store().Agent().GetWorkgroup(id)
	if err != nil {
		return nil, model.NewAppError("GetWorkgroup", "app.workgroup.get_workgroup.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return wg, nil
}

// DeleteWorkgroup soft-deletes a workgroup.
func (a *App) DeleteWorkgroup(rctx request.CTX, id string) *model.AppError {
	if err := a.Srv().Store().Agent().DeleteWorkgroup(id); err != nil {
		return model.NewAppError("DeleteWorkgroup", "app.workgroup.delete_workgroup.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Workgroup provisioning
// ---------------------------------------------------------------------------

// ProvisionWorkgroup creates a complete workgroup from a template or custom spec:
//  1. Creates bot users for each agent spec
//  2. Creates the three channels (general, internal, reports)
//  3. Saves AgentDefinition records
//  4. Saves the Workgroup record with status=active
func (a *App) ProvisionWorkgroup(rctx request.CTX, req *model.ProvisionWorkgroupRequest) (*model.Workgroup, *model.AppError) {
	// Resolve template
	var tmpl *model.WorkgroupTemplate
	if req.TemplateId != "" {
		t, err := a.GetWorkgroupTemplate(req.TemplateId)
		if err != nil {
			return nil, err
		}
		tmpl = t
	}

	// Build display name / name from template or request
	name := req.Name
	displayName := req.DisplayName
	wgType := model.WorkgroupTypeDepartment
	if tmpl != nil {
		if name == "" {
			name = tmpl.Name
		}
		if displayName == "" {
			displayName = tmpl.DisplayName
		}
		if tmpl.Name == "executive" {
			wgType = model.WorkgroupTypeExecutive
		}
	}
	if name == "" || req.TeamId == "" {
		return nil, model.NewAppError("ProvisionWorkgroup", "app.workgroup.provision.missing_params", nil, "", http.StatusBadRequest)
	}

	// Determine agent specs to use
	agentSpecs := req.AgentSpecs
	if len(agentSpecs) == 0 && tmpl != nil {
		agentSpecs = tmpl.AgentSpecs
	}

	// Create a stub workgroup record (provisioning status)
	wg := &model.Workgroup{
		TeamId:      req.TeamId,
		Name:        name,
		DisplayName: displayName,
		Type:        wgType,
		Status:      model.WorkgroupStatusProvisioning,
	}

	// Create the three channels
	channelIds, appErr := a.createWorkgroupChannels(rctx, req.TeamId, name, displayName)
	if appErr != nil {
		return nil, appErr
	}
	wg.ChannelIds = *channelIds

	// Save workgroup
	saved, saveErr := a.Srv().Store().Agent().SaveWorkgroup(wg)
	if saveErr != nil {
		return nil, model.NewAppError("ProvisionWorkgroup", "app.workgroup.save.app_error", nil, saveErr.Error(), http.StatusInternalServerError)
	}

	// Create agent definitions (and their bot users)
	var headAgentId string
	for _, spec := range agentSpecs {
		def, agentErr := a.createAgentFromSpec(rctx, saved.Id, req.TeamId, spec, wg.ChannelIds)
		if agentErr != nil {
			rctx.Logger().Warn("Failed to create agent from spec",
				mlog.String("workgroup_id", saved.Id),
				mlog.String("agent_name", spec.Name),
				mlog.Err(agentErr),
			)
			continue
		}
		if spec.Role == model.AgentRoleHead || spec.Role == model.AgentRoleExecutive {
			headAgentId = def.Id
		}
	}

	// Update workgroup with head agent ID and active status
	saved.HeadAgentId = headAgentId
	saved.Status = model.WorkgroupStatusActive
	updated, updateErr := a.Srv().Store().Agent().UpdateWorkgroup(saved)
	if updateErr != nil {
		return nil, model.NewAppError("ProvisionWorkgroup", "app.workgroup.update.app_error", nil, updateErr.Error(), http.StatusInternalServerError)
	}

	return updated, nil
}

// createWorkgroupChannels creates the three standard channels for a workgroup.
func (a *App) createWorkgroupChannels(rctx request.CTX, teamId, name, displayName string) (*model.WorkgroupChannelIDs, *model.AppError) {
	general, err := a.createWorkgroupChannel(rctx, teamId,
		fmt.Sprintf("%s-general", name),
		fmt.Sprintf("%s — General", displayName),
		model.ChannelTypeOpen,
	)
	if err != nil {
		return nil, err
	}

	internal, err := a.createWorkgroupChannel(rctx, teamId,
		fmt.Sprintf("%s-internal", name),
		fmt.Sprintf("%s — Internal", displayName),
		model.ChannelTypePrivate,
	)
	if err != nil {
		return nil, err
	}

	reports, err := a.createWorkgroupChannel(rctx, teamId,
		fmt.Sprintf("%s-reports", name),
		fmt.Sprintf("%s — Reports", displayName),
		model.ChannelTypePrivate,
	)
	if err != nil {
		return nil, err
	}

	return &model.WorkgroupChannelIDs{
		General:  general.Id,
		Internal: internal.Id,
		Reports:  reports.Id,
	}, nil
}

// createWorkgroupChannel is a thin wrapper around CreateChannel that handles
// the idempotency case where the channel already exists.
func (a *App) createWorkgroupChannel(rctx request.CTX, teamId, name, displayName string, channelType model.ChannelType) (*model.Channel, *model.AppError) {
	channel := &model.Channel{
		TeamId:      teamId,
		Name:        name,
		DisplayName: displayName,
		Type:        channelType,
	}
	ch, err := a.CreateChannel(rctx, channel, false)
	if err != nil {
		// If already exists, fetch it
		if err.Id == "store.sql_channel.save_channel.exists.app_error" {
			existing, fetchErr := a.GetChannelByName(rctx, name, teamId, false)
			if fetchErr != nil {
				return nil, fetchErr
			}
			return existing, nil
		}
		return nil, err
	}
	return ch, nil
}

// createAgentFromSpec creates a bot user and AgentDefinition for an AgentSpec.
func (a *App) createAgentFromSpec(rctx request.CTX, workgroupId, teamId string, spec model.AgentSpec, channels model.WorkgroupChannelIDs) (*model.AgentDefinition, *model.AppError) {
	// Determine model ID based on role
	modelId := defaultSpecialistModelID
	if spec.Role == model.AgentRoleHead || spec.Role == model.AgentRoleExecutive {
		modelId = defaultHeadModelID
	}

	// Create bot user
	bot := &model.Bot{
		Username:    sanitizeBotUsername(fmt.Sprintf("%s-agent-%s", workgroupId[:8], spec.Name)),
		DisplayName: spec.Name,
		Description: fmt.Sprintf("%s agent for workgroup", spec.Role),
	}
	createdBot, botErr := a.CreateBot(rctx, bot)
	if botErr != nil {
		return nil, botErr
	}

	// Add bot to team so it can post
	if _, err := a.AddTeamMember(rctx, teamId, createdBot.UserId); err != nil {
		rctx.Logger().Warn("Failed to add agent bot to team",
			mlog.String("bot_user_id", createdBot.UserId),
			mlog.String("team_id", teamId),
			mlog.Err(err),
		)
	}

	// Add bot to all three workgroup channels
	for _, chId := range []string{channels.General, channels.Internal, channels.Reports} {
		if chId == "" {
			continue
		}
		ch, chErr := a.GetChannel(rctx, chId)
		if chErr != nil {
			rctx.Logger().Warn("Failed to get channel for agent bot membership",
				mlog.String("channel_id", chId),
				mlog.Err(chErr),
			)
			continue
		}
		if _, err := a.AddChannelMember(rctx, createdBot.UserId, ch, ChannelMemberOpts{}); err != nil {
			rctx.Logger().Warn("Failed to add agent bot to channel",
				mlog.String("bot_user_id", createdBot.UserId),
				mlog.String("channel_id", chId),
				mlog.Err(err),
			)
		}
	}

	// Save AgentDefinition
	def := &model.AgentDefinition{
		WorkgroupId:  workgroupId,
		Role:         spec.Role,
		BotUserId:    createdBot.UserId,
		DisplayName:  spec.Name,
		SystemPrompt: spec.SystemPrompt,
		ModelId:      modelId,
		Tools:        spec.Tools,
		Capabilities: spec.Capabilities,
		MemoryConfig: model.MemoryConfig{
			Enabled:       true,
			MaxGlobalKeys: 100,
			SemanticTopK:  5,
			ExpireDays:    90,
		},
	}

	saved, saveErr := a.Srv().Store().Agent().SaveAgentDefinition(def)
	if saveErr != nil {
		return nil, model.NewAppError("createAgentFromSpec", "app.workgroup.save_agent_definition.app_error", nil, saveErr.Error(), http.StatusInternalServerError)
	}

	// Register with the capability router so route_to_capability can find this agent
	if ars := a.AgentRuntimeService(); ars != nil {
		ars.RegisterAgentForRouting(saved)
	}

	return saved, nil
}

// sanitizeBotUsername converts a string to a valid bot username.
func sanitizeBotUsername(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			result = append(result, c)
		} else if c >= 'A' && c <= 'Z' {
			result = append(result, c+32) // to lower
		} else {
			result = append(result, '-')
		}
	}
	if len(result) > 64 {
		result = result[:64]
	}
	return string(result)
}

// ---------------------------------------------------------------------------
// Coordination channels
// ---------------------------------------------------------------------------

// CreateCoordinationChannel creates a ChannelTypeAgentDirect channel between two workgroups
// and adds both head-agent bot users as members. It also adds any observer user IDs supplied.
// The channel ID is stored in both workgroups' CoordinationChannels map.
func (a *App) CreateCoordinationChannel(
	rctx request.CTX,
	wg1 *model.Workgroup,
	wg2 *model.Workgroup,
	botUserId1, botUserId2 string,
	observerUserIds []string,
) (string, *model.AppError) {
	name := fmt.Sprintf("agentcoord-%s-%s", wg1.Id[:8], wg2.Id[:8])
	displayName := fmt.Sprintf("%s ↔ %s Coordination", wg1.DisplayName, wg2.DisplayName)

	ch, appErr := a.createWorkgroupChannel(rctx, wg1.TeamId, name, displayName, model.ChannelTypeAgentDirect)
	if appErr != nil {
		return "", appErr
	}

	// Add both head-agent bot users
	for _, uid := range []string{botUserId1, botUserId2} {
		if uid == "" {
			continue
		}
		if _, err := a.AddChannelMember(rctx, uid, ch, ChannelMemberOpts{}); err != nil {
			rctx.Logger().Warn("CreateCoordinationChannel: failed to add bot",
				mlog.String("user_id", uid), mlog.Err(err))
		}
	}

	// Add observers (e.g. admins running the demo)
	for _, uid := range observerUserIds {
		if uid == "" {
			continue
		}
		if _, err := a.AddChannelMember(rctx, uid, ch, ChannelMemberOpts{}); err != nil {
			rctx.Logger().Warn("CreateCoordinationChannel: failed to add observer",
				mlog.String("user_id", uid), mlog.Err(err))
		}
	}

	// Persist channel ID on both workgroups
	wg1.ChannelIds.SetCoordinationChannel(wg2.Id, ch.Id)
	if _, err := a.Srv().Store().Agent().UpdateWorkgroup(wg1); err != nil {
		rctx.Logger().Warn("CreateCoordinationChannel: failed to update wg1", mlog.Err(err))
	}
	wg2.ChannelIds.SetCoordinationChannel(wg1.Id, ch.Id)
	if _, err := a.Srv().Store().Agent().UpdateWorkgroup(wg2); err != nil {
		rctx.Logger().Warn("CreateCoordinationChannel: failed to update wg2", mlog.Err(err))
	}

	return ch.Id, nil
}

// GetAgentCoordinationChannels returns all ChannelTypeAgentDirect channels for the team.
func (a *App) GetAgentCoordinationChannels(rctx request.CTX, teamId string) ([]*model.Channel, *model.AppError) {
	channels, err := a.Srv().Store().Agent().GetAgentCoordinationChannels(teamId)
	if err != nil {
		return nil, model.NewAppError("GetAgentCoordinationChannels", "app.agent.get_coordination_channels.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return channels, nil
}

// ---------------------------------------------------------------------------
// Demo setup
// ---------------------------------------------------------------------------

// ProvisionDemoSetup creates a minimal two-workgroup demo:
//   Executive (1 agent, OpenClaw)  ↔  Sales (1 head + 2 specialists, OpenClaw)
// with a ChannelTypeAgentDirect coordination channel between them.
// observerUserIds are added to every agent channel so the admin can observe all dialogue.
func (a *App) ProvisionDemoSetup(rctx request.CTX, req *model.DemoSetupRequest) (*model.DemoSetupResponse, *model.AppError) {
	llmServiceId := req.LLMServiceId
	if llmServiceId == "" {
		llmServiceId = "openclaw"
	}

	// Build agent specs — override model to use the requested LLM service
	execSpecs := []model.AgentSpec{{
		Role:         model.AgentRoleExecutive,
		Name:         "CEO",
		SystemPrompt: executiveSystemPrompt,
		Capabilities: []string{"strategy", "coordination", "delegation"},
		Tools:        []string{"search_posts", "get_channel", "create_post", "delegate_task", "route_to_capability"},
	}}

	salesSpecs := []model.AgentSpec{
		{
			Role:         model.AgentRoleHead,
			Name:         "Sales Head",
			SystemPrompt: salesHeadSystemPrompt,
			Capabilities: []string{"sales", "account_management", "pipeline"},
			Tools:        []string{"search_posts", "get_channel", "create_post", "delegate_task", "route_to_capability"},
		},
		{
			Role:         model.AgentRoleSpecialist,
			Name:         "Prospecting Specialist",
			SystemPrompt: prospectingSpecialistSystemPrompt,
			Capabilities: []string{"lead_generation", "prospecting", "outreach"},
			Tools:        []string{"search_posts", "create_post"},
		},
		{
			Role:         model.AgentRoleSpecialist,
			Name:         "Proposal Writer",
			SystemPrompt: proposalWriterSystemPrompt,
			Capabilities: []string{"proposal_writing", "rfp_response", "pricing"},
			Tools:        []string{"search_posts", "create_post"},
		},
	}

	// Provision executive workgroup
	execWg, appErr := a.ProvisionWorkgroup(rctx, &model.ProvisionWorkgroupRequest{
		TemplateId: "", // custom specs below
		TeamId:     req.TeamId,
		Name:       "executive",
		DisplayName: "Executive Office",
		AgentSpecs: execSpecs,
	})
	if appErr != nil {
		return nil, appErr
	}

	// Provision sales workgroup
	salesWg, appErr := a.ProvisionWorkgroup(rctx, &model.ProvisionWorkgroupRequest{
		TemplateId:  "",
		TeamId:      req.TeamId,
		Name:        "sales",
		DisplayName: "Sales",
		AgentSpecs:  salesSpecs,
	})
	if appErr != nil {
		return nil, appErr
	}

	// Override LLM service ID on all created agent definitions to use the requested backend
	if llmServiceId != "" {
		for _, wgId := range []string{execWg.Id, salesWg.Id} {
			defs, _ := a.Srv().Store().Agent().GetAgentDefinitionsByWorkgroup(wgId)
			for _, def := range defs {
				def.LLMServiceId = llmServiceId
				if _, err := a.Srv().Store().Agent().UpdateAgentDefinition(def); err != nil {
					rctx.Logger().Warn("ProvisionDemoSetup: failed to update LLM service",
						mlog.String("agent_id", def.Id), mlog.Err(err))
				}
			}
		}
	}

	// Look up bot user IDs for head agents to add them to the coordination channel
	execBotUserId := a.getHeadAgentBotUserId(rctx, execWg.HeadAgentId)
	salesBotUserId := a.getHeadAgentBotUserId(rctx, salesWg.HeadAgentId)

	// Create coordination channel
	coordChannelId, appErr := a.CreateCoordinationChannel(rctx, execWg, salesWg,
		execBotUserId, salesBotUserId, req.ObserverUserIds)
	if appErr != nil {
		rctx.Logger().Warn("ProvisionDemoSetup: failed to create coordination channel", mlog.Err(appErr))
		coordChannelId = "" // non-fatal
	}

	// Also add observers to each workgroup's general/internal/reports channels
	for _, obsId := range req.ObserverUserIds {
		for _, chId := range []string{
			execWg.ChannelIds.General, execWg.ChannelIds.Internal, execWg.ChannelIds.Reports,
			salesWg.ChannelIds.General, salesWg.ChannelIds.Internal, salesWg.ChannelIds.Reports,
		} {
			if chId == "" {
				continue
			}
			ch, err := a.GetChannel(rctx, chId)
			if err != nil {
				continue
			}
			if _, err := a.AddChannelMember(rctx, obsId, ch, ChannelMemberOpts{}); err != nil {
				rctx.Logger().Warn("ProvisionDemoSetup: failed to add observer to channel",
					mlog.String("user_id", obsId), mlog.String("channel_id", chId), mlog.Err(err))
			}
		}
	}

	// Collect specialist agent IDs for the response
	var specialistIds []string
	salesDefs, _ := a.Srv().Store().Agent().GetAgentDefinitionsByWorkgroup(salesWg.Id)
	for _, d := range salesDefs {
		if d.Role == model.AgentRoleSpecialist {
			specialistIds = append(specialistIds, d.Id)
		}
	}

	return &model.DemoSetupResponse{
		ExecutiveWorkgroup:      execWg,
		SalesWorkgroup:          salesWg,
		CoordinationChannelId:   coordChannelId,
		ExecutiveAgentId:        execWg.HeadAgentId,
		SalesHeadAgentId:        salesWg.HeadAgentId,
		SalesSpecialistAgentIds: specialistIds,
	}, nil
}

// getHeadAgentBotUserId resolves an agent definition ID to its BotUserId.
func (a *App) getHeadAgentBotUserId(rctx request.CTX, agentDefId string) string {
	if agentDefId == "" {
		return ""
	}
	def, err := a.Srv().Store().Agent().GetAgentDefinition(agentDefId)
	if err != nil {
		return ""
	}
	return def.BotUserId
}

// ---------------------------------------------------------------------------
// AgentDefinition CRUD
// ---------------------------------------------------------------------------

// GetAgentDefinition returns an agent definition by ID.
func (a *App) GetAgentDefinition(rctx request.CTX, id string) (*model.AgentDefinition, *model.AppError) {
	def, err := a.Srv().Store().Agent().GetAgentDefinition(id)
	if err != nil {
		return nil, model.NewAppError("GetAgentDefinition", "app.agent_definition.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return def, nil
}

// ListAllAgentDefinitions returns all non-deleted agent definitions across all workgroups.
func (a *App) ListAllAgentDefinitions(rctx request.CTX) ([]*model.AgentDefinition, *model.AppError) {
	defs, err := a.Srv().Store().Agent().ListAllAgentDefinitions()
	if err != nil {
		return nil, model.NewAppError("ListAllAgentDefinitions", "app.agent_definition.list_all.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return defs, nil
}

// GetAgentDefinitionsByWorkgroup returns all agent definitions for a workgroup.
func (a *App) GetAgentDefinitionsByWorkgroup(rctx request.CTX, workgroupId string) ([]*model.AgentDefinition, *model.AppError) {
	defs, err := a.Srv().Store().Agent().GetAgentDefinitionsByWorkgroup(workgroupId)
	if err != nil {
		return nil, model.NewAppError("GetAgentDefinitionsByWorkgroup", "app.agent_definition.list.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return defs, nil
}

// CreateAgentDefinition creates a new agent definition.
func (a *App) CreateAgentDefinition(rctx request.CTX, def *model.AgentDefinition) (*model.AgentDefinition, *model.AppError) {
	if err := def.IsValid(); err != nil {
		return nil, err
	}
	saved, storeErr := a.Srv().Store().Agent().SaveAgentDefinition(def)
	if storeErr != nil {
		return nil, model.NewAppError("CreateAgentDefinition", "app.agent_definition.save.app_error", nil, storeErr.Error(), http.StatusInternalServerError)
	}
	return saved, nil
}

// PatchAgentDefinition applies a partial update to an agent definition.
func (a *App) PatchAgentDefinition(rctx request.CTX, id string, patch *model.PatchAgentDefinition) (*model.AgentDefinition, *model.AppError) {
	existing, appErr := a.GetAgentDefinition(rctx, id)
	if appErr != nil {
		return nil, appErr
	}
	existing.Patch(patch)
	updated, err := a.Srv().Store().Agent().UpdateAgentDefinition(existing)
	if err != nil {
		return nil, model.NewAppError("PatchAgentDefinition", "app.agent_definition.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return updated, nil
}

// DeleteAgentDefinition soft-deletes an agent definition.
func (a *App) DeleteAgentDefinition(rctx request.CTX, id string) *model.AppError {
	if err := a.Srv().Store().Agent().DeleteAgentDefinition(id); err != nil {
		return model.NewAppError("DeleteAgentDefinition", "app.agent_definition.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

// ---------------------------------------------------------------------------
// First-run seeding
// ---------------------------------------------------------------------------

// SeedDefaultWorkgroupsIfNeeded provisions Executive Office + Marketing on first run
// if no workgroups exist for the primary team.
func (a *App) SeedDefaultWorkgroupsIfNeeded(rctx request.CTX, teamId string) {
	existing, err := a.Srv().Store().Agent().GetWorkgroupsForTeam(teamId)
	if err != nil {
		rctx.Logger().Error("Failed to check existing workgroups for seeding", mlog.Err(err))
		return
	}
	if len(existing) > 0 {
		return // already provisioned
	}

	rctx.Logger().Info("Seeding default workgroups: Executive Office and Marketing")

	for _, templateId := range []string{"executive", "marketing"} {
		if _, provErr := a.ProvisionWorkgroup(rctx, &model.ProvisionWorkgroupRequest{
			TemplateId: templateId,
			TeamId:     teamId,
		}); provErr != nil {
			rctx.Logger().Error("Failed to provision default workgroup",
				mlog.String("template_id", templateId),
				mlog.Err(provErr),
			)
		}
	}
}

// ---------------------------------------------------------------------------
// System prompts for built-in templates
// ---------------------------------------------------------------------------

const executiveSystemPrompt = `You are the Executive AI of this company. Your role is to:
- Coordinate strategy across all departments
- Route high-level directives to the appropriate department head agents
- Synthesize reports from departments for human stakeholders
- Make strategic decisions and prioritize company initiatives

When you receive a request, determine which department(s) should handle it and delegate accordingly.
Always keep the human informed of progress and provide clear summaries of outcomes.`

const marketingHeadSystemPrompt = `You are the Head of Marketing. Your role is to:
- Oversee all marketing initiatives and campaigns
- Coordinate with campaign strategists and content writers
- Report marketing results to the Executive
- Respond to human requests about marketing strategy, campaigns, and brand

When complex tasks arise, delegate to your specialist agents (Campaign Strategist, Content Writer).
Always provide clear, actionable responses and maintain brand consistency.`

const campaignStrategistSystemPrompt = `You are a Campaign Strategist. Your expertise includes:
- Developing marketing campaign strategies
- Market research and competitive analysis
- Campaign performance analysis and optimization
- Target audience identification and segmentation

Provide data-driven, strategic recommendations. Be specific with metrics and timelines.`

const contentWriterSystemPrompt = `You are a Content Writer. Your expertise includes:
- Writing compelling marketing copy and content
- Social media content creation
- Blog posts, email campaigns, and ad copy
- Brand voice and messaging consistency

Create engaging, on-brand content that resonates with the target audience.`

const engineeringHeadSystemPrompt = `You are the Head of Engineering. Your role is to:
- Oversee technical architecture and engineering processes
- Coordinate backend and frontend engineering efforts
- Report technical progress and blockers to the Executive
- Respond to technical questions and strategy requests

Delegate specific technical tasks to Backend or Frontend Engineers as appropriate.`

const backendEngineerSystemPrompt = `You are a Backend Engineer. Your expertise includes:
- Server-side architecture and API design
- Database design and optimization
- Performance, scalability, and security
- Code review and technical documentation

Provide technically sound, practical engineering guidance.`

const frontendEngineerSystemPrompt = `You are a Frontend Engineer. Your expertise includes:
- React/TypeScript UI development
- Component architecture and state management
- Performance optimization and accessibility
- UI/UX implementation best practices

Provide clear, implementable frontend solutions.`

const salesHeadSystemPrompt = `You are the Head of Sales. Your role is to:
- Oversee the entire sales pipeline and process
- Coordinate account executives and SDRs
- Report pipeline and revenue metrics to the Executive
- Respond to questions about sales strategy and customer accounts

Delegate prospecting tasks to SDRs and account management to Account Executives.`

const accountExecutiveSystemPrompt = `You are an Account Executive. Your expertise includes:
- Managing and growing customer relationships
- Negotiating contracts and closing deals
- Upselling and cross-selling to existing accounts
- Customer success and retention

Focus on building long-term customer value and revenue.`

const prospectingSpecialistSystemPrompt = `You are a Prospecting Specialist. Your expertise includes:
- Identifying and researching high-potential target accounts
- Crafting personalized outreach sequences (email, LinkedIn, phone)
- Qualifying leads against ICP (Ideal Customer Profile) criteria
- Scheduling discovery calls and handoffs to account executives

Focus on top-of-funnel efficiency. When you complete a task, post a concise summary to the channel.`

const proposalWriterSystemPrompt = `You are a Proposal Writer. Your expertise includes:
- Crafting compelling sales proposals and RFP responses
- Translating customer pain points into value-focused narratives
- Building pricing structures and ROI justifications
- Editing and polishing proposal documents for clarity

Produce proposal content that directly addresses the customer's stated requirements.`

const sdrSystemPrompt = `You are a Sales Development Representative. Your expertise includes:
- Prospecting and lead generation
- Qualifying inbound and outbound leads
- Setting discovery calls and demos
- CRM data management

Focus on identifying and qualifying high-quality leads for the Account Executive team.`

const financeHeadSystemPrompt = `You are the Head of Finance. Your role is to:
- Oversee financial planning, budgeting, and reporting
- Coordinate financial analysts and accountants
- Provide financial insights to the Executive
- Ensure financial compliance and controls

Delegate detailed analysis to Financial Analysts and accounting tasks to Accountants.`

const financialAnalystSystemPrompt = `You are a Financial Analyst. Your expertise includes:
- Financial modeling and forecasting
- Variance analysis and performance reporting
- Investment analysis and ROI calculations
- Budget planning and scenario modeling

Provide data-driven financial insights and clear visualizations of financial data.`

const accountantSystemPrompt = `You are an Accountant. Your expertise includes:
- General ledger maintenance and bookkeeping
- Financial statement preparation
- Tax compliance and reporting
- Accounts payable/receivable management

Ensure accuracy, compliance, and timely financial reporting.`

const researchHeadSystemPrompt = `You are the Head of Research & Development. Your role is to:
- Lead research initiatives and innovation
- Coordinate research analysts and technical researchers
- Report research findings to the Executive
- Identify emerging trends and opportunities

Delegate literature reviews to Research Analysts and technical experiments to Technical Researchers.`

const researchAnalystSystemPrompt = `You are a Research Analyst. Your expertise includes:
- Systematic literature review and synthesis
- Quantitative and qualitative research methods
- Data analysis and statistical interpretation
- Research report writing

Produce rigorous, well-cited research analyses.`

const technicalResearcherSystemPrompt = `You are a Technical Researcher. Your expertise includes:
- Technical research and feasibility studies
- Proof-of-concept development
- Technology evaluation and benchmarking
- Technical documentation

Provide practical, technically grounded research outcomes.`

const productHeadSystemPrompt = `You are the Head of Product. Your role is to:
- Own the product vision, roadmap, and prioritization
- Coordinate product managers and UX researchers
- Report product progress and metrics to the Executive
- Respond to product strategy and feature questions

Delegate requirements work to Product Managers and user research to UX Researchers.`

const productManagerSystemPrompt = `You are a Product Manager. Your expertise includes:
- Writing clear product requirements and user stories
- Feature specification and acceptance criteria
- Roadmap planning and sprint coordination
- Stakeholder communication and alignment

Produce clear, developer-ready specifications.`

const uxResearcherSystemPrompt = `You are a UX Researcher. Your expertise includes:
- User interview design and facilitation
- Usability testing and analysis
- User journey mapping and personas
- Translating research into actionable design insights

Provide human-centered insights that drive better product decisions.`

const manufacturingHeadSystemPrompt = `You are the Head of Manufacturing. Your role is to:
- Oversee production operations and quality standards
- Coordinate production planners and quality engineers
- Report production metrics to the Executive
- Identify process improvements and efficiencies

Delegate scheduling to Production Planners and quality issues to Quality Engineers.`

const productionPlannerSystemPrompt = `You are a Production Planner. Your expertise includes:
- Production scheduling and capacity planning
- Materials requirements planning (MRP)
- Lead time optimization
- Production bottleneck identification

Provide practical, data-driven production plans.`

const qualityEngineerSystemPrompt = `You are a Quality Engineer. Your expertise includes:
- Quality control processes and standards (ISO, Six Sigma)
- Inspection and testing protocols
- Root cause analysis and corrective actions
- Compliance and regulatory requirements

Ensure products meet the highest quality standards.`

const supplyChainHeadSystemPrompt = `You are the Head of Supply Chain. Your role is to:
- Oversee end-to-end supply chain operations
- Coordinate procurement and logistics teams
- Report supply chain metrics to the Executive
- Identify cost reduction and reliability improvements

Delegate vendor management to Procurement Specialists and shipping to Logistics Coordinators.`

const procurementSpecialistSystemPrompt = `You are a Procurement Specialist. Your expertise includes:
- Vendor sourcing and evaluation
- Contract negotiation and management
- Purchase order management
- Supplier relationship management

Optimize procurement costs while maintaining quality and reliability.`

const logisticsCoordinatorSystemPrompt = `You are a Logistics Coordinator. Your expertise includes:
- Shipping and freight management
- Inventory control and warehousing
- Order fulfillment and tracking
- Last-mile delivery optimization

Ensure efficient, cost-effective movement of goods.`
