// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useSelector} from 'react-redux';

import type {AgentDefinition} from '@mattermost/types/agent_tasks';

import type {GlobalState} from 'types/store';

import AgentDetailPanel from './agent_detail_panel';
import AgentGrid from './agent_grid';
import AgentStatusBar from './agent_status_bar';

export default function AgentCommandCenter() {
    const [selectedAgentId, setSelectedAgentId] = useState('');
    const [filterWorkgroupId, setFilterWorkgroupId] = useState('');

    // Toggle full-screen body class
    useEffect(() => {
        document.body.classList.add('agent-command-center');
        return () => {
            document.body.classList.remove('agent-command-center');
        };
    }, []);

    const selectedDefinition: AgentDefinition | undefined = useSelector((state: GlobalState) => {
        if (!selectedAgentId) { return undefined; }
        const list: AgentDefinition[] = (state.entities as any).agentDefinitions?.list ?? [];
        return list.find((d) => d.id === selectedAgentId);
    });

    return (
        <div className='agent-command-center-root'>
            <AgentStatusBar
                selectedWorkgroupId={filterWorkgroupId}
                onWorkgroupChange={setFilterWorkgroupId}
                onNewTask={() => {}}
            />
            <div className='acc-main'>
                <AgentGrid
                    filterWorkgroupId={filterWorkgroupId}
                    selectedAgentId={selectedAgentId}
                    onSelectAgent={setSelectedAgentId}
                />
                {selectedDefinition && (
                    <AgentDetailPanel
                        definition={selectedDefinition}
                        onClose={() => setSelectedAgentId('')}
                    />
                )}
            </div>
        </div>
    );
}
