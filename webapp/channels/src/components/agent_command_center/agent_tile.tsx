// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useRef, useEffect} from 'react';
import {useSelector} from 'react-redux';

import type {AgentDefinition, AgentMetrics, AgentTask, AgentTaskStatus} from '@mattermost/types/agent_tasks';

import {getActiveTaskForAgent, getAgentMetricsById} from 'mattermost-redux/selectors/entities/agent_tasks';
import type {GlobalState} from 'types/store';

type Props = {
    definition: AgentDefinition;
    workgroupName?: string;
    selected: boolean;
    onSelect: () => void;
    onDispatch?: (agentId: string, input: string) => void;
};

function statusLabel(status: AgentTaskStatus | undefined): string {
    switch (status) {
    case 'running': return 'Running';
    case 'claimed': return 'Claimed';
    case 'pending': return 'Pending';
    case 'awaiting_subtask': return 'Waiting';
    case 'complete': return 'Complete';
    case 'failed': return 'Failed';
    default: return 'Idle';
    }
}

function statusClass(status: AgentTaskStatus | undefined): string {
    switch (status) {
    case 'running': return 'running';
    case 'claimed':
    case 'pending':
    case 'awaiting_subtask': return 'pending';
    case 'complete': return 'complete';
    case 'failed': return 'failed';
    default: return 'idle';
    }
}

function isActive(status: AgentTaskStatus | undefined): boolean {
    return status === 'running' || status === 'claimed' || status === 'pending' || status === 'awaiting_subtask';
}

function formatMs(ms: number): string {
    if (ms < 1000) { return `${ms}ms`; }
    return `${(ms / 1000).toFixed(1)}s`;
}

function initials(name: string): string {
    return name.
        split(/[\s_-]+/).
        slice(0, 2).
        map((w) => w[0] ?? '').
        join('').
        toUpperCase();
}

export default function AgentTile({definition, workgroupName, selected, onSelect, onDispatch}: Props) {
    const activeTask: AgentTask | undefined = useSelector((state: GlobalState) =>
        getActiveTaskForAgent(state, definition.id),
    );
    const metrics: AgentMetrics | undefined = useSelector((state: GlobalState) =>
        getAgentMetricsById(state, definition.id),
    );

    const [showDispatch, setShowDispatch] = useState(false);
    const [dispatchText, setDispatchText] = useState('');
    const dispatchRef = useRef<HTMLTextAreaElement>(null);

    useEffect(() => {
        if (showDispatch) {
            dispatchRef.current?.focus();
        }
    }, [showDispatch]);

    const tileClass = [
        'acc-tile',
        selected ? 'acc-tile--selected' : '',
        activeTask ? `acc-tile--${statusClass(activeTask.status)}` : '',
    ].filter(Boolean).join(' ');

    const status = activeTask?.status;
    const streamText = activeTask?.output?.text ?? '';
    const preview = streamText.length > 200 ? '…' + streamText.slice(-200) : streamText;

    // Recent tool calls from events (we don't fetch events here; show from task output metadata)
    const toolCalls: string[] = [];
    if (activeTask?.output?.metadata) {
        const meta = activeTask.output.metadata as any;
        if (Array.isArray(meta.tool_calls)) {
            toolCalls.push(...(meta.tool_calls as string[]).slice(-4));
        }
    }

    function handleDispatchSubmit() {
        if (!dispatchText.trim()) { return; }
        onDispatch?.(definition.id, dispatchText.trim());
        setDispatchText('');
        setShowDispatch(false);
    }

    return (
        <div
            className={tileClass}
            onClick={onSelect}
            role='button'
            tabIndex={0}
            onKeyDown={(e) => e.key === 'Enter' && onSelect()}
        >
            <div className='acc-tile__header'>
                <div className='acc-tile__avatar'>{initials(definition.display_name)}</div>
                <span className='acc-tile__name'>{definition.display_name}</span>
                {workgroupName && (
                    <span className='acc-tile__workgroup-badge'>{workgroupName}</span>
                )}
                <span className={`acc-tile__status-pill acc-tile__status-pill--${statusClass(status)}`}>
                    <span className={`acc-tile__status-dot${isActive(status) ? ' acc-tile__status-dot--pulse' : ''}`}/>
                    {statusLabel(status)}
                </span>
            </div>

            <div className='acc-tile__body'>
                {preview || <span style={{opacity: 0.4}}>{'No active task'}</span>}
            </div>

            {toolCalls.length > 0 && (
                <div className='acc-tile__tool-badges'>
                    {toolCalls.map((tc, i) => (
                        <span key={i} className='acc-tile__tool-badge'>{tc}</span>
                    ))}
                </div>
            )}

            <div
                className='acc-tile__footer'
                onClick={(e) => e.stopPropagation()}
            >
                {metrics && (
                    <>
                        <FooterStat value={String(metrics.tasks_submitted)} label='Tasks'/>
                        <FooterStat value={String(metrics.tokens_total.toLocaleString())} label='Tokens'/>
                        {metrics.avg_latency_ms > 0 && (
                            <FooterStat value={formatMs(metrics.avg_latency_ms)} label='Avg Lat'/>
                        )}
                    </>
                )}
                <button
                    className='acc-tile__dispatch-btn'
                    type='button'
                    onClick={(e) => {
                        e.stopPropagation();
                        setShowDispatch((v) => !v);
                    }}
                >
                    {'Dispatch'}
                </button>
            </div>

            {showDispatch && (
                <div
                    className='acc-dispatch-form'
                    onClick={(e) => e.stopPropagation()}
                >
                    <textarea
                        ref={dispatchRef}
                        value={dispatchText}
                        onChange={(e) => setDispatchText(e.target.value)}
                        placeholder='Enter task instructions…'
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                                handleDispatchSubmit();
                            }
                            if (e.key === 'Escape') {
                                setShowDispatch(false);
                            }
                        }}
                    />
                    <div className='acc-dispatch-form__actions'>
                        <button
                            type='button'
                            onClick={() => setShowDispatch(false)}
                            style={{padding: '4px 10px', cursor: 'pointer', border: '1px solid rgba(0,0,0,0.2)', borderRadius: 4, background: 'none'}}
                        >
                            {'Cancel'}
                        </button>
                        <button
                            type='button'
                            className='acc-tile__dispatch-btn'
                            onClick={handleDispatchSubmit}
                            disabled={!dispatchText.trim()}
                        >
                            {'Send'}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}

function FooterStat({value, label}: {value: string; label: string}) {
    return (
        <div className='acc-tile__footer-stat'>
            <span className='acc-tile__footer-stat-value'>{value}</span>
            <span className='acc-tile__footer-stat-label'>{label}</span>
        </div>
    );
}
