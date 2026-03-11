// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {bindClientFunc} from './helpers';

import {AgentTypes} from '../action_types';
import {Client4} from '../client';

export function getAgents() {
    return bindClientFunc({
        clientFunc: Client4.getAgents,
        onSuccess: [AgentTypes.RECEIVED_AGENTS],
        onFailure: AgentTypes.AGENTS_FAILURE,
        onRequest: AgentTypes.AGENTS_REQUEST,
    });
}

export function getAgentsStatus() {
    return bindClientFunc({
        clientFunc: Client4.getAgentsStatus,
        onSuccess: [AgentTypes.RECEIVED_AGENTS_STATUS],
        onFailure: AgentTypes.AGENTS_STATUS_FAILURE,
        onRequest: AgentTypes.AGENTS_STATUS_REQUEST,
    });
}

export function getLLMServices() {
    return bindClientFunc({
        clientFunc: Client4.getLLMServices,
        onSuccess: [AgentTypes.RECEIVED_LLM_SERVICES],
        onFailure: AgentTypes.LLM_SERVICES_FAILURE,
        onRequest: AgentTypes.LLM_SERVICES_REQUEST,
    });
}

export function getAgentDefinitions() {
    return bindClientFunc({
        clientFunc: Client4.getAgentDefinitions,
        onSuccess: [AgentTypes.RECEIVED_AGENT_DEFINITIONS],
        onFailure: AgentTypes.AGENT_DEFINITIONS_FAILURE,
        onRequest: AgentTypes.AGENT_DEFINITIONS_REQUEST,
    });
}

export function getAgentMetrics() {
    return bindClientFunc({
        clientFunc: Client4.getAgentMetrics,
        onSuccess: [AgentTypes.RECEIVED_AGENT_METRICS],
        onFailure: AgentTypes.AGENT_METRICS_FAILURE,
        onRequest: AgentTypes.AGENT_METRICS_REQUEST,
    });
}

export function getActiveAgentTasks(agentId: string) {
    return bindClientFunc({
        clientFunc: Client4.getActiveAgentTasks,
        params: [agentId] as [string],
        onSuccess: [AgentTypes.RECEIVED_ACTIVE_TASKS],
        onFailure: AgentTypes.ACTIVE_TASKS_FAILURE,
        onRequest: AgentTypes.ACTIVE_TASKS_REQUEST,
    });
}

export function patchAgentDefinition(id: string, patch: Record<string, unknown>) {
    return bindClientFunc({
        clientFunc: Client4.patchAgentDefinition,
        params: [id, patch] as [string, Record<string, unknown>],
        onSuccess: [AgentTypes.RECEIVED_AGENT_DEFINITION],
        onFailure: AgentTypes.PATCH_AGENT_DEFINITION_FAILURE,
        onRequest: AgentTypes.PATCH_AGENT_DEFINITION_REQUEST,
    });
}
