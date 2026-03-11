// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useCallback} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {AgentDefinition} from '@mattermost/types/agent_tasks';
import type {Workgroup} from '@mattermost/types/workgroups';

import {getAgentDefinitions} from 'mattermost-redux/actions/agents';
import {Client4} from 'mattermost-redux/client';
import type {GlobalState} from 'types/store';

import AgentTile from './agent_tile';

type Props = {
    filterWorkgroupId: string;
    selectedAgentId: string;
    onSelectAgent: (id: string) => void;
};

function gridClass(count: number): string {
    if (count <= 1) { return 'acc-grid--1col'; }
    if (count <= 4) { return 'acc-grid--2col'; }
    return 'acc-grid--3col';
}

export default function AgentGrid({filterWorkgroupId, selectedAgentId, onSelectAgent}: Props) {
    const dispatch = useDispatch();

    const definitions: AgentDefinition[] = useSelector((state: GlobalState) =>
        (state.entities as any).agentDefinitions?.list ?? [],
    );

    const workgroupsById: Record<string, Workgroup> = useSelector((state: GlobalState) =>
        state.entities.workgroups?.byId ?? {},
    );

    useEffect(() => {
        dispatch(getAgentDefinitions() as any);
    }, [dispatch]);

    const filtered = filterWorkgroupId ?
        definitions.filter((d) => d.workgroup_id === filterWorkgroupId) :
        definitions;

    function handleDispatch(agentId: string, input: string) {
        Client4.submitAgentTask(agentId, input);
    }

    if (filtered.length === 0) {
        return (
            <div className='acc-grid'>
                <div className='acc-empty-state'>
                    <div className='acc-empty-state__title'>{'No Agents Found'}</div>
                    <div className='acc-empty-state__subtitle'>
                        {'Provision a workgroup or create an agent definition to get started.'}
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className={`acc-grid ${gridClass(filtered.length)}`}>
            {filtered.map((def) => {
                const wg = def.workgroup_id ? workgroupsById[def.workgroup_id] : undefined;
                return (
                    <AgentTile
                        key={def.id}
                        definition={def}
                        workgroupName={wg?.display_name ?? wg?.name}
                        selected={def.id === selectedAgentId}
                        onSelect={() => onSelectAgent(def.id === selectedAgentId ? '' : def.id)}
                        onDispatch={handleDispatch}
                    />
                );
            })}
        </div>
    );
}
