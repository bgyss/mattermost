// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export const AgentTaskStatusPending = 'pending';
export const AgentTaskStatusClaimed = 'claimed';
export const AgentTaskStatusRunning = 'running';
export const AgentTaskStatusAwaitingSubtask = 'awaiting_subtask';
export const AgentTaskStatusComplete = 'complete';
export const AgentTaskStatusFailed = 'failed';

export type AgentTaskStatus =
    | 'pending'
    | 'claimed'
    | 'running'
    | 'awaiting_subtask'
    | 'complete'
    | 'failed';

export type AgentTaskInput = {
    text: string;
    metadata?: Record<string, unknown>;
};

export type AgentTaskOutput = {
    text: string;
    metadata?: Record<string, unknown>;
};

export type AgentTask = {
    id: string;
    agent_id: string;
    parent_task_id: string;
    root_task_id: string;
    channel_id: string;
    thread_root_post_id: string;
    request_post_id: string;
    response_post_id: string;
    input: AgentTaskInput;
    output: AgentTaskOutput;
    status: AgentTaskStatus;
    priority: number;
    delegation_chain: string[];
    tokens_used: number;
    latency_ms: number;
    error_msg?: string;
    create_at: number;
    update_at: number;
    claimed_at: number;
    complete_at: number;
};

export type AgentTaskEventType =
    | 'thinking'
    | 'tool_call'
    | 'tool_result'
    | 'delegation'
    | 'completion'
    | 'error'
    | 'token_chunk';

export type AgentTaskEvent = {
    id: string;
    task_id: string;
    agent_id: string;
    event_type: AgentTaskEventType;
    payload: Record<string, unknown>;
    create_at: number;
};

export type AgentDefinition = {
    id: string;
    workgroup_id: string;
    role: 'head' | 'specialist' | 'executive';
    bot_user_id: string;
    display_name: string;
    system_prompt: string;
    llm_service_id: string;
    model_id: string;
    model_parameters: {
        temperature?: number;
        max_tokens?: number;
        top_p?: number;
    };
    tools: string[];
    capabilities: string[];
    max_concurrency: number;
    pool_size: number;
    memory_config: {
        enabled: boolean;
        max_global_keys: number;
        semantic_top_k: number;
        expire_days: number;
    };
    owner_user_id: string;
    create_at: number;
    update_at: number;
    delete_at: number;
};

export type AgentMemory = {
    id: string;
    agent_id: string;
    scope: 'global' | 'task' | 'user';
    scope_id: string;
    key: string;
    value_text: string;
    expire_at: number;
    create_at: number;
    update_at: number;
};

export type AgentTasksState = {
    byId: Record<string, AgentTask>;
    eventsByTask: Record<string, string[]>; // taskId -> eventIds
    eventsById: Record<string, AgentTaskEvent>;
};

export type AgentMetrics = {
    agent_id: string;
    tasks_submitted: number;
    tasks_completed: number;
    tasks_failed: number;
    tokens_total: number;
    avg_latency_ms: number;
};
