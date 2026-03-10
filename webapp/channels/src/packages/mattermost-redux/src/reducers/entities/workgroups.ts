// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {combineReducers} from 'redux';

import type {Workgroup, WorkgroupTemplate} from '@mattermost/types/workgroups';

import type {MMReduxAction} from 'mattermost-redux/action_types';

export const WorkgroupActionTypes = {
    RECEIVED_WORKGROUPS: 'RECEIVED_WORKGROUPS',
    RECEIVED_WORKGROUP: 'RECEIVED_WORKGROUP',
    WORKGROUP_DELETED: 'WORKGROUP_DELETED',
    RECEIVED_WORKGROUP_TEMPLATES: 'RECEIVED_WORKGROUP_TEMPLATES',
} as const;

export type WorkgroupsEntityState = {
    byId: Record<string, Workgroup>;
    byTeam: Record<string, string[]>;
    templates: WorkgroupTemplate[];
};

function byId(state: Record<string, Workgroup> = {}, action: MMReduxAction): Record<string, Workgroup> {
    switch (action.type) {
    case WorkgroupActionTypes.RECEIVED_WORKGROUP: {
        const wg: Workgroup = action.data;
        return {...state, [wg.id]: wg};
    }
    case WorkgroupActionTypes.RECEIVED_WORKGROUPS: {
        const wgs: Workgroup[] = action.data;
        const next = {...state};
        for (const wg of wgs) {
            next[wg.id] = wg;
        }
        return next;
    }
    case WorkgroupActionTypes.WORKGROUP_DELETED: {
        const next = {...state};
        delete next[action.data.id];
        return next;
    }
    default:
        return state;
    }
}

function byTeam(state: Record<string, string[]> = {}, action: MMReduxAction): Record<string, string[]> {
    switch (action.type) {
    case WorkgroupActionTypes.RECEIVED_WORKGROUPS: {
        const wgs: Workgroup[] = action.data;
        if (!wgs.length) {
            return state;
        }
        const teamId = wgs[0].team_id;
        return {...state, [teamId]: wgs.map((w) => w.id)};
    }
    case WorkgroupActionTypes.RECEIVED_WORKGROUP: {
        const wg: Workgroup = action.data;
        const existing = state[wg.team_id] ?? [];
        if (existing.includes(wg.id)) {
            return state;
        }
        return {...state, [wg.team_id]: [...existing, wg.id]};
    }
    case WorkgroupActionTypes.WORKGROUP_DELETED: {
        const wg: Workgroup = action.data;
        const existing = state[wg.team_id] ?? [];
        return {...state, [wg.team_id]: existing.filter((id) => id !== wg.id)};
    }
    default:
        return state;
    }
}

function templates(state: WorkgroupTemplate[] = [], action: MMReduxAction): WorkgroupTemplate[] {
    switch (action.type) {
    case WorkgroupActionTypes.RECEIVED_WORKGROUP_TEMPLATES:
        return action.data ?? [];
    default:
        return state;
    }
}

export default combineReducers({
    byId,
    byTeam,
    templates,
});
