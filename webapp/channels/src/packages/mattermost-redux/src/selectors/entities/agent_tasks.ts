// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AgentDefinition, AgentMemory, AgentMetrics, AgentTask, AgentTaskEvent} from '@mattermost/types/agent_tasks';
import type {GlobalState} from '@mattermost/types/store';

// ---- AgentTask selectors ----

export function getAllAgentTasks(state: GlobalState): Record<string, AgentTask> {
    return state.entities.agentTasks?.byId ?? {};
}

export function getTasksByAgent(state: GlobalState, agentId: string): AgentTask[] {
    const byId = getAllAgentTasks(state);
    return Object.values(byId).filter((t) => t.agent_id === agentId);
}

export function getActiveTaskForAgent(state: GlobalState, agentId: string): AgentTask | undefined {
    const tasks = getTasksByAgent(state, agentId);
    return tasks.find(
        (t) => t.status === 'running' || t.status === 'claimed' || t.status === 'pending',
    );
}

export function getTaskEventsByTask(state: GlobalState, taskId: string): AgentTaskEvent[] {
    const eventIds = state.entities.agentTasks?.eventsByTask?.[taskId] ?? [];
    const eventsById = state.entities.agentTasks?.eventsById ?? {};
    return eventIds.map((id) => eventsById[id]).filter(Boolean);
}

// ---- AgentDefinition selectors (stored in workgroups state) ----

export function getAgentDefinitionList(state: GlobalState): AgentDefinition[] {
    return (state.entities as any).agentDefinitions?.list ?? [];
}

export function getAgentDefinitionById(state: GlobalState, id: string): AgentDefinition | undefined {
    return getAgentDefinitionList(state).find((d) => d.id === id);
}

// ---- AgentMetrics selectors ----

export function getAgentMetricsAll(state: GlobalState): AgentMetrics[] {
    return (state.entities as any).agentMetrics?.list ?? [];
}

export function getAgentMetricsById(state: GlobalState, agentId: string): AgentMetrics | undefined {
    return getAgentMetricsAll(state).find((m) => m.agent_id === agentId);
}

// ---- AgentMemory selectors ----

export function getAgentMemoryList(state: GlobalState, agentId: string): AgentMemory[] {
    return ((state.entities as any).agentMemory?.byAgent?.[agentId]) ?? [];
}
