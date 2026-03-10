// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useSelector} from 'react-redux';
import styled from 'styled-components';

import type {AgentTask} from '@mattermost/types/agent_tasks';

import type {GlobalState} from 'types/store';

type Props = {
    rootTaskId: string;
};

// Build a tree structure from a flat list of tasks
type TaskNode = AgentTask & {children: TaskNode[]};

function buildTree(tasks: AgentTask[], rootId: string): TaskNode | null {
    const byId = new Map<string, TaskNode>();
    for (const t of tasks) {
        byId.set(t.id, {...t, children: []});
    }
    let root: TaskNode | null = null;
    for (const node of byId.values()) {
        if (node.id === rootId || node.parent_task_id === '') {
            root = root ?? node;
        } else {
            const parent = byId.get(node.parent_task_id);
            parent?.children.push(node);
        }
    }
    return root;
}

const statusColor = (status: string) => {
    switch (status) {
    case 'complete': return 'rgb(var(--semantic-color-success-rgb, 62,175,100))';
    case 'failed': return 'rgb(var(--semantic-color-danger-rgb, 210,75,78))';
    case 'running': return 'rgb(var(--semantic-color-info-rgb, 28,88,217))';
    default: return 'rgba(var(--center-channel-color-rgb), 0.4)';
    }
};

const Tree = styled.div`
    font-size: 12px;
    font-family: var(--font-family);
    padding: 8px 0;
`;

const NodeWrap = styled.div<{depth: number}>`
    display: flex;
    flex-direction: column;
    padding-left: ${({depth}) => depth * 20}px;
`;

const NodeRow = styled.div`
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    border-radius: 4px;
    cursor: default;

    &:hover {
        background: rgba(var(--center-channel-color-rgb), 0.06);
    }
`;

const StatusDot = styled.span<{status: string}>`
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    background: ${({status}) => statusColor(status)};
`;

const ConnectorLine = styled.span`
    margin-left: 10px;
    margin-right: 2px;
    color: rgba(var(--center-channel-color-rgb), 0.3);
    font-size: 14px;
    flex-shrink: 0;
`;

const AgentName = styled.span`
    font-weight: 600;
    color: rgba(var(--center-channel-color-rgb), 0.85);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
`;

const TaskId = styled.span`
    font-size: 10px;
    opacity: 0.4;
    font-family: monospace;
`;

const StatusLabel = styled.span<{status: string}>`
    font-size: 10px;
    color: ${({status}) => statusColor(status)};
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.3px;
`;

const Tokens = styled.span`
    font-size: 10px;
    opacity: 0.5;
`;

function TaskNodeView({node, depth}: {node: TaskNode; depth: number}) {
    return (
        <NodeWrap depth={depth}>
            <NodeRow>
                {depth > 0 && <ConnectorLine>{'└'}</ConnectorLine>}
                <StatusDot status={node.status}/>
                <AgentName>{node.agent_id.substring(0, 8)}</AgentName>
                <StatusLabel status={node.status}>{node.status}</StatusLabel>
                {node.tokens_used > 0 && (
                    <Tokens>{node.tokens_used.toLocaleString()}{' tok'}</Tokens>
                )}
                <TaskId>{node.id.substring(0, 6)}</TaskId>
            </NodeRow>
            {node.children.map((child) => (
                <TaskNodeView
                    key={child.id}
                    node={child}
                    depth={depth + 1}
                />
            ))}
        </NodeWrap>
    );
}

export default function AgentDelegationTree({rootTaskId}: Props) {
    const tasks = useSelector((state: GlobalState) => {
        const byId = state.entities.agentTasks?.byId ?? {};
        return Object.values(byId).filter(
            (t) => t.root_task_id === rootTaskId || t.id === rootTaskId,
        );
    });

    const tree = useMemo(() => buildTree(tasks, rootTaskId), [tasks, rootTaskId]);

    if (!tree) {
        return null;
    }

    // Only show the tree if there's actually delegation (more than 1 node)
    if (tasks.length <= 1) {
        return null;
    }

    return (
        <Tree>
            <TaskNodeView
                node={tree}
                depth={0}
            />
        </Tree>
    );
}
