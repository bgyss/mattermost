// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import keyMirror from 'mattermost-redux/utils/key_mirror';

export default keyMirror({
    RECEIVED_AGENTS: null,
    AGENTS_REQUEST: null,
    AGENTS_FAILURE: null,

    RECEIVED_AGENTS_STATUS: null,
    AGENTS_STATUS_REQUEST: null,
    AGENTS_STATUS_FAILURE: null,

    RECEIVED_LLM_SERVICES: null,
    LLM_SERVICES_REQUEST: null,
    LLM_SERVICES_FAILURE: null,

    // AgentDefinition (multi-agent orchestration platform)
    RECEIVED_AGENT_DEFINITIONS: null,
    AGENT_DEFINITIONS_REQUEST: null,
    AGENT_DEFINITIONS_FAILURE: null,

    RECEIVED_AGENT_DEFINITION: null,
    PATCH_AGENT_DEFINITION_REQUEST: null,
    PATCH_AGENT_DEFINITION_FAILURE: null,

    RECEIVED_AGENT_METRICS: null,
    AGENT_METRICS_REQUEST: null,
    AGENT_METRICS_FAILURE: null,

    RECEIVED_ACTIVE_TASKS: null,
    ACTIVE_TASKS_REQUEST: null,
    ACTIVE_TASKS_FAILURE: null,
});
