// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';
import {useSelector} from 'react-redux';
import styled, {keyframes} from 'styled-components';

import type {AgentTaskEvent} from '@mattermost/types/agent_tasks';

import type {GlobalState} from 'types/store';

type Props = {
    taskId: string;
};

function selectEventsForTask(state: GlobalState, taskId: string): AgentTaskEvent[] {
    const eventIds = state.entities.agentTasks?.eventsByTask?.[taskId] ?? [];
    const eventsById = state.entities.agentTasks?.eventsById ?? {};
    return eventIds.map((id) => eventsById[id]).filter(Boolean);
}

const slideDown = keyframes`
    from { opacity: 0; transform: translateY(-4px); }
    to   { opacity: 1; transform: translateY(0); }
`;

const Panel = styled.div`
    margin: 8px 0;
    padding: 8px 12px;
    border-radius: 6px;
    background: rgba(var(--center-channel-color-rgb), 0.04);
    border: 1px solid rgba(var(--center-channel-color-rgb), 0.1);
    font-size: 12px;
    font-family: monospace;
    max-height: 300px;
    overflow-y: auto;
`;

const Row = styled.div<{isError?: boolean}>`
    display: flex;
    gap: 8px;
    padding: 3px 0;
    border-bottom: 1px solid rgba(var(--center-channel-color-rgb), 0.06);
    animation: ${slideDown} 0.15s ease-out;
    color: ${({isError}) => isError ? 'rgb(var(--semantic-color-danger-rgb, 210,75,78))' : 'rgba(var(--center-channel-color-rgb), 0.72)'};

    &:last-child { border-bottom: none; }
`;

const EventType = styled.span`
    flex-shrink: 0;
    width: 90px;
    font-weight: 600;
    font-size: 10px;
    letter-spacing: 0.5px;
    text-transform: uppercase;
    opacity: 0.6;
    padding-top: 1px;
`;

const EventContent = styled.span`
    flex: 1;
    word-break: break-word;
    white-space: pre-wrap;
    line-height: 1.4;
`;

function formatPayload(eventType: string, payload: Record<string, unknown>): string {
    switch (eventType) {
    case 'tool_call':
        return `${payload.tool}(${payload.args})`;
    case 'tool_result':
        return payload.is_error
            ? `✗ ${payload.tool}: ${payload.result}`
            : `✓ ${payload.tool}: ${String(payload.result).substring(0, 120)}`;
    case 'thinking':
        return String(payload.message ?? '');
    case 'delegation':
        return `→ ${payload.delegate_to}`;
    case 'completion':
        return `${payload.tokens_input ?? 0} in / ${payload.tokens_output ?? 0} out tokens · ${payload.latency_ms ?? 0}ms`;
    case 'error':
        return String(payload.message ?? payload.error ?? '');
    default:
        return JSON.stringify(payload);
    }
}

const typeColors: Record<string, string> = {
    thinking: 'rgba(var(--semantic-color-info-rgb, 28,88,217), 0.8)',
    tool_call: 'rgba(var(--semantic-color-warning-rgb, 210,150,30), 0.9)',
    tool_result: 'rgba(62,175,100, 0.9)',
    delegation: 'rgba(150,80,200, 0.9)',
    completion: 'rgba(var(--semantic-color-success-rgb, 62,175,100), 1)',
    error: 'rgb(var(--semantic-color-danger-rgb, 210,75,78))',
};

export default function AgentThinkingPanel({taskId}: Props) {
    const events = useSelector((state: GlobalState) => selectEventsForTask(state, taskId));
    const bottomRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        bottomRef.current?.scrollIntoView({behavior: 'smooth'});
    }, [events.length]);

    if (events.length === 0) {
        return null;
    }

    return (
        <Panel>
            {events.map((evt) => (
                <Row
                    key={evt.id}
                    isError={evt.event_type === 'error'}
                >
                    <EventType style={{color: typeColors[evt.event_type] ?? undefined}}>
                        {evt.event_type.replace('_', ' ')}
                    </EventType>
                    <EventContent>
                        {formatPayload(evt.event_type, evt.payload ?? {})}
                    </EventContent>
                </Row>
            ))}
            <div ref={bottomRef}/>
        </Panel>
    );
}
