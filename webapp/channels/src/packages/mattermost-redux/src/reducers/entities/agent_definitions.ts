// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {combineReducers} from 'redux';

import type {AgentDefinition, AgentMemory, AgentMetrics} from '@mattermost/types/agent_tasks';

import type {MMReduxAction} from 'mattermost-redux/action_types';

// ---- AgentDefinition list ----

function list(state: AgentDefinition[] = [], action: MMReduxAction): AgentDefinition[] {
    switch (action.type) {
    case 'RECEIVED_AGENT_DEFINITIONS': {
        const defs: AgentDefinition[] = action.data;
        return defs ?? [];
    }
    case 'RECEIVED_AGENT_DEFINITION': {
        const def: AgentDefinition = action.data;
        const idx = state.findIndex((d) => d.id === def.id);
        if (idx === -1) {
            return [...state, def];
        }
        const next = [...state];
        next[idx] = def;
        return next;
    }
    default:
        return state;
    }
}

export const agentDefinitions = combineReducers({list});

// ---- AgentMetrics ----

function metricsList(state: AgentMetrics[] = [], action: MMReduxAction): AgentMetrics[] {
    switch (action.type) {
    case 'RECEIVED_AGENT_METRICS': {
        const metrics: AgentMetrics[] = action.data;
        return metrics ?? [];
    }
    default:
        return state;
    }
}

export const agentMetrics = combineReducers({list: metricsList});

// ---- AgentMemory per-agent ----

function byAgent(state: Record<string, AgentMemory[]> = {}, action: MMReduxAction): Record<string, AgentMemory[]> {
    switch (action.type) {
    case 'RECEIVED_AGENT_MEMORY': {
        const {agentId, memories} = action.data as {agentId: string; memories: AgentMemory[]};
        return {...state, [agentId]: memories};
    }
    default:
        return state;
    }
}

export const agentMemory = combineReducers({byAgent});
