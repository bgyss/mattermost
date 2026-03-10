// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Workgroup types
export const WorkgroupTypeDepartment = 'department';
export const WorkgroupTypeExecutive = 'executive';

// Workgroup statuses
export const WorkgroupStatusProvisioning = 'provisioning';
export const WorkgroupStatusActive = 'active';
export const WorkgroupStatusSuspended = 'suspended';

export type WorkgroupChannelIDs = {
    general: string;
    internal: string;
    reports: string;
};

export type Workgroup = {
    id: string;
    team_id: string;
    name: string;
    display_name: string;
    type: 'department' | 'executive';
    head_agent_id: string;
    status: 'provisioning' | 'active' | 'suspended';
    channel_ids: WorkgroupChannelIDs;
    create_at: number;
    update_at: number;
    delete_at: number;
};

export type AgentSpec = {
    role: 'head' | 'specialist' | 'executive';
    name: string;
    system_prompt: string;
    capabilities: string[];
    tools: string[];
};

export type WorkgroupTemplate = {
    id: string;
    name: string;
    display_name: string;
    agent_specs: AgentSpec[];
    default_llm_tier: 'standard' | 'premium';
};

export type ProvisionWorkgroupRequest = {
    template_id: string;
    team_id: string;
    name?: string;
    display_name?: string;
    agent_specs?: AgentSpec[];
};

export type WorkgroupsState = {
    byId: Record<string, Workgroup>;
    byTeam: Record<string, string[]>; // teamId -> workgroupIds
    templates: WorkgroupTemplate[];
};
