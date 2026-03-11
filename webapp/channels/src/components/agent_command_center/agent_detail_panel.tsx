// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {AgentDefinition, AgentMemory, AgentTask} from '@mattermost/types/agent_tasks';

import {getTasksByAgent} from 'mattermost-redux/selectors/entities/agent_tasks';
import {patchAgentDefinition, getActiveAgentTasks} from 'mattermost-redux/actions/agents';
import {Client4} from 'mattermost-redux/client';
import type {GlobalState} from 'types/store';
import AgentDelegationTree from 'components/agent_delegation_tree';

type Tab = 'active' | 'history' | 'tree' | 'memory' | 'settings';

type Props = {
    definition: AgentDefinition;
    onClose: () => void;
};

export default function AgentDetailPanel({definition, onClose}: Props) {
    const [tab, setTab] = useState<Tab>('active');
    const dispatch = useDispatch();

    return (
        <div className='acc-detail-panel'>
            <div className='acc-detail-panel__header'>
                <span>{definition.display_name}</span>
                <button
                    className='acc-detail-panel__close-btn'
                    type='button'
                    onClick={onClose}
                    aria-label='Close panel'
                >
                    {'×'}
                </button>
            </div>

            <div className='acc-detail-panel__tabs'>
                {(['active', 'history', 'tree', 'memory', 'settings'] as Tab[]).map((t) => (
                    <button
                        key={t}
                        type='button'
                        className={`acc-detail-panel__tab${tab === t ? ' acc-detail-panel__tab--active' : ''}`}
                        onClick={() => setTab(t)}
                    >
                        {t === 'active' ? 'Active Tasks' :
                            t === 'history' ? 'History' :
                            t === 'tree' ? 'Delegation Tree' :
                            t === 'memory' ? 'Memory' : 'Settings'}
                    </button>
                ))}
            </div>

            <div className='acc-detail-panel__content'>
                {tab === 'active' && <ActiveTasksTab agentId={definition.id}/>}
                {tab === 'history' && <HistoryTab agentId={definition.id}/>}
                {tab === 'tree' && <TreeTab agentId={definition.id}/>}
                {tab === 'memory' && <MemoryTab agentId={definition.id}/>}
                {tab === 'settings' && (
                    <SettingsTab
                        definition={definition}
                        onSave={(patch) => dispatch(patchAgentDefinition(definition.id, patch) as any)}
                    />
                )}
            </div>
        </div>
    );
}

// ---- Active Tasks tab ----
function ActiveTasksTab({agentId}: {agentId: string}) {
    const dispatch = useDispatch();
    const tasks: AgentTask[] = useSelector((state: GlobalState) => getTasksByAgent(state, agentId));

    const activeTasks = tasks.filter(
        (t) => t.status === 'running' || t.status === 'claimed' || t.status === 'pending' || t.status === 'awaiting_subtask',
    );

    useEffect(() => {
        dispatch(getActiveAgentTasks(agentId) as any);
    }, [dispatch, agentId]);

    if (activeTasks.length === 0) {
        return (
            <div className='acc-empty-state'>
                <div className='acc-empty-state__title'>{'No Active Tasks'}</div>
                <div className='acc-empty-state__subtitle'>{'This agent is idle.'}</div>
            </div>
        );
    }

    return (
        <div>
            {activeTasks.map((task) => <TaskRow key={task.id} task={task}/>)}
        </div>
    );
}

// ---- History tab ----
function HistoryTab({agentId}: {agentId: string}) {
    const tasks: AgentTask[] = useSelector((state: GlobalState) => getTasksByAgent(state, agentId));
    const done = tasks.filter((t) => t.status === 'complete' || t.status === 'failed');

    if (done.length === 0) {
        return (
            <div className='acc-empty-state'>
                <div className='acc-empty-state__title'>{'No Task History'}</div>
                <div className='acc-empty-state__subtitle'>{'Completed tasks will appear here.'}</div>
            </div>
        );
    }

    return (
        <div>
            {done.slice().reverse().map((task) => <TaskRow key={task.id} task={task}/>)}
        </div>
    );
}

function TaskRow({task}: {task: AgentTask}) {
    return (
        <div className='acc-task-row'>
            <div className='acc-task-row__header'>
                <span>{task.status.toUpperCase()}</span>
                <span style={{fontSize: 11, opacity: 0.5, marginLeft: 'auto'}}>
                    {new Date(task.create_at).toLocaleTimeString()}
                </span>
            </div>
            <div className='acc-task-row__input'>{task.input?.text ?? '—'}</div>
            <div className='acc-task-row__stats'>
                {task.tokens_used > 0 && <span>{task.tokens_used.toLocaleString()} tok</span>}
                {task.latency_ms > 0 && <span>{(task.latency_ms / 1000).toFixed(1)}s</span>}
            </div>
        </div>
    );
}

// ---- Delegation Tree tab ----
function TreeTab({agentId}: {agentId: string}) {
    const tasks: AgentTask[] = useSelector((state: GlobalState) => getTasksByAgent(state, agentId));

    // Find any task that is a root (no parent) for this agent
    const rootTask = tasks.find((t) => !t.parent_task_id || t.parent_task_id === t.root_task_id);

    if (!rootTask) {
        return (
            <div className='acc-empty-state'>
                <div className='acc-empty-state__title'>{'No Delegation Tree'}</div>
                <div className='acc-empty-state__subtitle'>{'Subtask delegation will appear here.'}</div>
            </div>
        );
    }

    return <AgentDelegationTree rootTaskId={rootTask.root_task_id || rootTask.id}/>;
}

// ---- Memory tab ----
function MemoryTab({agentId}: {agentId: string}) {
    const [memories, setMemories] = useState<AgentMemory[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        setLoading(true);
        Client4.getAgentMemory(agentId).
            then((data) => setMemories(data)).
            catch(() => setMemories([])).
            finally(() => setLoading(false));
    }, [agentId]);

    if (loading) {
        return <div style={{padding: 16, opacity: 0.6}}>{'Loading memory…'}</div>;
    }

    if (memories.length === 0) {
        return (
            <div className='acc-empty-state'>
                <div className='acc-empty-state__title'>{'No Memory Entries'}</div>
                <div className='acc-empty-state__subtitle'>{'Agent memory will appear here.'}</div>
            </div>
        );
    }

    return (
        <div>
            {memories.map((mem) => (
                <div key={mem.id} className='acc-memory-row'>
                    <div className='acc-memory-row__key'>{mem.key}</div>
                    <div className='acc-memory-row__value'>{mem.value_text}</div>
                    <div className='acc-memory-row__scope'>{mem.scope}</div>
                </div>
            ))}
        </div>
    );
}

// ---- Settings tab ----
type SettingsTabProps = {
    definition: AgentDefinition;
    onSave: (patch: Partial<AgentDefinition>) => void;
};

function SettingsTab({definition, onSave}: SettingsTabProps) {
    const [displayName, setDisplayName] = useState(definition.display_name);
    const [systemPrompt, setSystemPrompt] = useState(definition.system_prompt);
    const [modelId, setModelId] = useState(definition.model_id);
    const [maxConcurrency, setMaxConcurrency] = useState(definition.max_concurrency);
    const [saving, setSaving] = useState(false);

    function handleSave() {
        setSaving(true);
        onSave({
            display_name: displayName,
            system_prompt: systemPrompt,
            model_id: modelId,
            max_concurrency: maxConcurrency,
        });
        setSaving(false);
    }

    return (
        <div className='acc-settings-form'>
            <label>
                {'Display Name'}
                <input
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                />
            </label>
            <label>
                {'Model ID'}
                <input
                    value={modelId}
                    onChange={(e) => setModelId(e.target.value)}
                />
            </label>
            <label>
                {'Max Concurrency'}
                <input
                    type='number'
                    min={1}
                    max={50}
                    value={maxConcurrency}
                    onChange={(e) => setMaxConcurrency(Number(e.target.value))}
                />
            </label>
            <label>
                {'System Prompt'}
                <textarea
                    value={systemPrompt}
                    onChange={(e) => setSystemPrompt(e.target.value)}
                />
            </label>
            <button
                type='button'
                className='acc-settings-save-btn'
                disabled={saving}
                onClick={handleSave}
            >
                {saving ? 'Saving…' : 'Save Changes'}
            </button>
        </div>
    );
}
