// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {combineReducers} from 'redux';

import type {AgentTask, AgentTaskEvent} from '@mattermost/types/agent_tasks';

import type {MMReduxAction} from 'mattermost-redux/action_types';

export const AgentTaskActionTypes = {
    RECEIVED_AGENT_TASK: 'RECEIVED_AGENT_TASK',
    RECEIVED_AGENT_TASK_TREE: 'RECEIVED_AGENT_TASK_TREE',
    RECEIVED_AGENT_TASK_EVENTS: 'RECEIVED_AGENT_TASK_EVENTS',
    RECEIVED_AGENT_TASK_EVENT: 'RECEIVED_AGENT_TASK_EVENT',
    AGENT_TASK_STATUS_UPDATED: 'AGENT_TASK_STATUS_UPDATED',
    AGENT_TOKEN_STREAM: 'AGENT_TOKEN_STREAM',
} as const;

function byId(state: Record<string, AgentTask> = {}, action: MMReduxAction): Record<string, AgentTask> {
    switch (action.type) {
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK:
    case AgentTaskActionTypes.AGENT_TASK_STATUS_UPDATED: {
        const task: AgentTask = action.data;
        return {...state, [task.id]: task};
    }
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK_TREE:
    case 'RECEIVED_ACTIVE_TASKS': {
        const tasks: AgentTask[] = action.data;
        const next = {...state};
        for (const task of tasks) {
            next[task.id] = task;
        }
        return next;
    }
    case AgentTaskActionTypes.AGENT_TOKEN_STREAM: {
        const {task_id, token} = action.data as {task_id: string; token: string};
        const existing = state[task_id];
        if (!existing) {
            return state;
        }
        return {
            ...state,
            [task_id]: {
                ...existing,
                output: {
                    ...existing.output,
                    text: (existing.output?.text ?? '') + token,
                },
            },
        };
    }
    default:
        return state;
    }
}

function eventsById(state: Record<string, AgentTaskEvent> = {}, action: MMReduxAction): Record<string, AgentTaskEvent> {
    switch (action.type) {
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK_EVENTS: {
        const events: AgentTaskEvent[] = action.data;
        const next = {...state};
        for (const e of events) {
            next[e.id] = e;
        }
        return next;
    }
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK_EVENT: {
        const e: AgentTaskEvent = action.data;
        return {...state, [e.id]: e};
    }
    default:
        return state;
    }
}

function eventsByTask(state: Record<string, string[]> = {}, action: MMReduxAction): Record<string, string[]> {
    switch (action.type) {
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK_EVENTS: {
        const events: AgentTaskEvent[] = action.data;
        if (!events.length) {
            return state;
        }
        const taskId = events[0].task_id;
        return {...state, [taskId]: events.map((e) => e.id)};
    }
    case AgentTaskActionTypes.RECEIVED_AGENT_TASK_EVENT: {
        const e: AgentTaskEvent = action.data;
        const existing = state[e.task_id] ?? [];
        if (existing.includes(e.id)) {
            return state;
        }
        return {...state, [e.task_id]: [...existing, e.id]};
    }
    default:
        return state;
    }
}

export default combineReducers({
    byId,
    eventsById,
    eventsByTask,
});
