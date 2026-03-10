// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState, useCallback} from 'react';
import styled from 'styled-components';

import type {AgentMetrics} from '@mattermost/types/agent_tasks';

import {Client4} from '@mattermost/client';

// ---------------------------------------------------------------------------
// Styled components
// ---------------------------------------------------------------------------

const Page = styled.div`
    padding: 24px;
    font-family: var(--font-family);
    max-width: 960px;
`;

const Header = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
`;

const Title = styled.h2`
    font-size: 18px;
    font-weight: 700;
    color: var(--center-channel-color);
    margin: 0;
`;

const RefreshBtn = styled.button`
    background: rgba(var(--button-bg-rgb), 0.1);
    border: 1px solid rgba(var(--button-bg-rgb), 0.3);
    border-radius: 4px;
    color: rgb(var(--button-bg-rgb));
    cursor: pointer;
    font-size: 13px;
    padding: 6px 14px;

    &:hover {
        background: rgba(var(--button-bg-rgb), 0.18);
    }
`;

const Table = styled.table`
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
`;

const Th = styled.th`
    text-align: left;
    padding: 8px 12px;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.4px;
    color: rgba(var(--center-channel-color-rgb), 0.6);
    border-bottom: 2px solid rgba(var(--center-channel-color-rgb), 0.1);
`;

const Td = styled.td`
    padding: 10px 12px;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.08);
    color: var(--center-channel-color);
    font-family: monospace;
`;

const AgentIdCell = styled(Td)`
    font-weight: 600;
    font-size: 12px;
`;

const StatusBar = styled.div<{ratio: number; color: string}>`
    height: 6px;
    border-radius: 3px;
    background: rgba(var(--center-channel-color-rgb), 0.08);
    position: relative;
    min-width: 80px;

    &::after {
        content: '';
        position: absolute;
        left: 0;
        top: 0;
        height: 100%;
        width: ${({ratio}) => Math.min(ratio * 100, 100)}%;
        border-radius: 3px;
        background: ${({color}) => color};
        transition: width 0.3s ease;
    }
`;

const NumberCell = styled(Td)`
    text-align: right;
`;

const ErrorNote = styled.p`
    color: rgb(var(--semantic-color-danger-rgb, 210,75,78));
    font-size: 13px;
`;

const EmptyNote = styled.p`
    color: rgba(var(--center-channel-color-rgb), 0.5);
    font-size: 13px;
`;

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

function formatTokens(n: number): string {
    if (n >= 1_000_000) {
        return `${(n / 1_000_000).toFixed(1)}M`;
    }
    if (n >= 1_000) {
        return `${(n / 1_000).toFixed(1)}K`;
    }
    return String(n);
}

function successRatio(m: AgentMetrics): number {
    if (m.tasks_submitted === 0) {
        return 0;
    }
    return m.tasks_completed / m.tasks_submitted;
}

export default function AgentTaskMetrics() {
    const [metrics, setMetrics] = useState<AgentMetrics[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    const load = useCallback(async () => {
        setLoading(true);
        setError('');
        try {
            const data = await Client4.doFetch<AgentMetrics[]>(
                `${Client4.getUrl()}/api/v4/agents/metrics`,
                {method: 'GET'},
            );
            // Sort descending by tasks_submitted so busiest agents appear first.
            data.sort((a, b) => b.tasks_submitted - a.tasks_submitted);
            setMetrics(data);
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : 'Failed to load metrics');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        load();
    }, [load]);

    return (
        <Page>
            <Header>
                <Title>{'Agent Runtime Metrics'}</Title>
                <RefreshBtn
                    onClick={load}
                    disabled={loading}
                >
                    {loading ? 'Loading…' : 'Refresh'}
                </RefreshBtn>
            </Header>

            {error && <ErrorNote>{error}</ErrorNote>}

            {!loading && !error && metrics.length === 0 && (
                <EmptyNote>{'No agent activity recorded since last server start.'}</EmptyNote>
            )}

            {metrics.length > 0 && (
                <Table>
                    <thead>
                        <tr>
                            <Th>{'Agent ID'}</Th>
                            <Th style={{textAlign: 'right'}}>{'Submitted'}</Th>
                            <Th style={{textAlign: 'right'}}>{'Completed'}</Th>
                            <Th style={{textAlign: 'right'}}>{'Failed'}</Th>
                            <Th>{'Success Rate'}</Th>
                            <Th style={{textAlign: 'right'}}>{'Tokens'}</Th>
                            <Th style={{textAlign: 'right'}}>{'Avg Latency'}</Th>
                        </tr>
                    </thead>
                    <tbody>
                        {metrics.map((m) => {
                            const ratio = successRatio(m);
                            return (
                                <tr key={m.agent_id}>
                                    <AgentIdCell>{m.agent_id.substring(0, 12)}</AgentIdCell>
                                    <NumberCell>{m.tasks_submitted.toLocaleString()}</NumberCell>
                                    <NumberCell>{m.tasks_completed.toLocaleString()}</NumberCell>
                                    <NumberCell
                                        style={{
                                            color: m.tasks_failed > 0 ?
                                                'rgb(var(--semantic-color-danger-rgb, 210,75,78))' :
                                                'inherit',
                                        }}
                                    >
                                        {m.tasks_failed.toLocaleString()}
                                    </NumberCell>
                                    <Td>
                                        <StatusBar
                                            ratio={ratio}
                                            color={ratio > 0.9 ?
                                                'rgb(var(--semantic-color-success-rgb, 62,175,100))' :
                                                ratio > 0.7 ?
                                                    'rgb(var(--semantic-color-warning-rgb, 255,165,0))' :
                                                    'rgb(var(--semantic-color-danger-rgb, 210,75,78))'}
                                        />
                                        <span style={{fontSize: 10, opacity: 0.7, marginLeft: 6}}>
                                            {Math.round(ratio * 100)}{'%'}
                                        </span>
                                    </Td>
                                    <NumberCell>{formatTokens(m.tokens_total)}</NumberCell>
                                    <NumberCell>{m.avg_latency_ms.toLocaleString()}{'ms'}</NumberCell>
                                </tr>
                            );
                        })}
                    </tbody>
                </Table>
            )}
        </Page>
    );
}
