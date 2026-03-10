// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';
import styled, {keyframes} from 'styled-components';

import type {GlobalState} from 'types/store';

type Props = {
    postId: string;
};

// Selector: find an agent task by its responsePostId
function selectTaskForPost(state: GlobalState, postId: string) {
    const tasks = state.entities.agentTasks?.byId;
    if (!tasks) {
        return null;
    }
    return Object.values(tasks).find((t) => t.response_post_id === postId) ?? null;
}

const pulse = keyframes`
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.4; }
`;

const Badge = styled.span<{status: string}>`
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.3px;
    margin-left: 8px;
    vertical-align: middle;
    background: ${({status}) => {
        switch (status) {
        case 'complete': return 'rgba(var(--semantic-color-success-rgb, 62,175,100), 0.15)';
        case 'failed': return 'rgba(var(--semantic-color-danger-rgb, 210,75,78), 0.15)';
        case 'running':
        case 'claimed': return 'rgba(var(--semantic-color-info-rgb, 28,88,217), 0.15)';
        default: return 'rgba(var(--center-channel-color-rgb), 0.08)';
        }
    }};
    color: ${({status}) => {
        switch (status) {
        case 'complete': return 'rgb(var(--semantic-color-success-rgb, 62,175,100))';
        case 'failed': return 'rgb(var(--semantic-color-danger-rgb, 210,75,78))';
        case 'running':
        case 'claimed': return 'rgb(var(--semantic-color-info-rgb, 28,88,217))';
        default: return 'rgba(var(--center-channel-color-rgb), 0.56)';
        }
    }};
`;

const Dot = styled.span<{animate: boolean}>`
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    animation: ${({animate}) => animate ? `${pulse} 1.4s ease-in-out infinite` : 'none'};
`;

const TokenCount = styled.span`
    opacity: 0.72;
    font-weight: 400;
    margin-left: 2px;
`;

function statusLabel(status: string): string {
    switch (status) {
    case 'pending': return 'Queued';
    case 'claimed': return 'Starting';
    case 'running': return 'Thinking';
    case 'awaiting_subtask': return 'Delegating';
    case 'complete': return 'Done';
    case 'failed': return 'Failed';
    default: return status;
    }
}

export default function AgentTaskStatus({postId}: Props) {
    const task = useSelector((state: GlobalState) => selectTaskForPost(state, postId));

    if (!task) {
        return null;
    }

    const isActive = task.status === 'running' || task.status === 'claimed' || task.status === 'pending';

    return (
        <Badge status={task.status}>
            <Dot animate={isActive}/>
            {statusLabel(task.status)}
            {task.tokens_used > 0 && (
                <TokenCount>
                    {'· '}
                    {task.tokens_used.toLocaleString()}
                    {' tok'}
                </TokenCount>
            )}
        </Badge>
    );
}
