// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useCallback} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {AgentMetrics} from '@mattermost/types/agent_tasks';
import type {Workgroup} from '@mattermost/types/workgroups';

import {getAgentMetrics as fetchAgentMetrics} from 'mattermost-redux/actions/agents';
import type {GlobalState} from 'types/store';

type Props = {
    selectedWorkgroupId: string;
    onWorkgroupChange: (id: string) => void;
    onNewTask: () => void;
};

function sumMetrics(metrics: AgentMetrics[]) {
    return metrics.reduce(
        (acc, m) => ({
            tasksToday: acc.tasksToday + m.tasks_submitted,
            tokensTotal: acc.tokensTotal + m.tokens_total,
            active: acc.active + (m.tasks_submitted - m.tasks_completed - m.tasks_failed > 0 ? 1 : 0),
            failed: acc.failed + m.tasks_failed,
        }),
        {tasksToday: 0, tokensTotal: 0, active: 0, failed: 0},
    );
}

function formatTokens(n: number): string {
    if (n >= 1_000_000) {
        return `${(n / 1_000_000).toFixed(1)}M`;
    }
    if (n >= 1_000) {
        return `${(n / 1_000).toFixed(1)}k`;
    }
    return String(n);
}

export default function AgentStatusBar({selectedWorkgroupId, onWorkgroupChange, onNewTask}: Props) {
    const dispatch = useDispatch();

    const metrics: AgentMetrics[] = useSelector((state: GlobalState) =>
        (state.entities as any).agentMetrics?.list ?? [],
    );

    const workgroups: Workgroup[] = useSelector((state: GlobalState) =>
        Object.values(state.entities.workgroups?.byId ?? {}),
    );

    const fetchMetrics = useCallback(() => {
        dispatch(fetchAgentMetrics() as any);
    }, [dispatch]);

    useEffect(() => {
        fetchMetrics();
        const id = setInterval(fetchMetrics, 30_000);
        return () => clearInterval(id);
    }, [fetchMetrics]);

    const totals = sumMetrics(metrics);

    return (
        <div className='acc-status-bar'>
            <Metric value={String(totals.tasksToday)} label='Tasks Today'/>
            <div className='acc-status-bar__divider'/>
            <Metric value={formatTokens(totals.tokensTotal)} label='Tokens'/>
            <div className='acc-status-bar__divider'/>
            <Metric value={String(totals.active)} label='Active'/>
            <div className='acc-status-bar__divider'/>
            <Metric value={String(totals.failed)} label='Failed'/>

            <select
                className='acc-status-bar__workgroup-select'
                value={selectedWorkgroupId}
                onChange={(e) => onWorkgroupChange(e.target.value)}
            >
                <option value=''>{'All Workgroups'}</option>
                {workgroups.map((wg) => (
                    <option key={wg.id} value={wg.id}>{wg.display_name || wg.name}</option>
                ))}
            </select>

            <button
                className='acc-status-bar__new-task-btn'
                onClick={onNewTask}
                type='button'
            >
                {'+ New Task'}
            </button>
        </div>
    );
}

function Metric({value, label}: {value: string; label: string}) {
    return (
        <div className='acc-status-bar__metric'>
            <span className='acc-status-bar__metric-value'>{value}</span>
            <span className='acc-status-bar__metric-label'>{label}</span>
        </div>
    );
}
