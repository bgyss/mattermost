// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkgroupChannelIDs_CoordinationChannels(t *testing.T) {
	t.Run("SetCoordinationChannel initialises nil map", func(t *testing.T) {
		ids := &WorkgroupChannelIDs{}
		ids.SetCoordinationChannel("wg2", "ch1")
		assert.Equal(t, "ch1", ids.CoordinationChannels["wg2"])
	})

	t.Run("GetCoordinationChannel returns empty string for nil map", func(t *testing.T) {
		ids := &WorkgroupChannelIDs{}
		assert.Equal(t, "", ids.GetCoordinationChannel("wg2"))
	})

	t.Run("GetCoordinationChannel returns empty string for missing key", func(t *testing.T) {
		ids := &WorkgroupChannelIDs{CoordinationChannels: map[string]string{"wg2": "ch1"}}
		assert.Equal(t, "", ids.GetCoordinationChannel("wg3"))
	})

	t.Run("SetCoordinationChannel overwrites existing value", func(t *testing.T) {
		ids := &WorkgroupChannelIDs{CoordinationChannels: map[string]string{"wg2": "ch1"}}
		ids.SetCoordinationChannel("wg2", "ch2")
		assert.Equal(t, "ch2", ids.GetCoordinationChannel("wg2"))
	})

	t.Run("round-trips through JSON without breaking existing fields", func(t *testing.T) {
		original := WorkgroupChannelIDs{
			General:  "g1",
			Internal: "i1",
			Reports:  "r1",
		}
		original.SetCoordinationChannel("peer1", "coord1")

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded WorkgroupChannelIDs
		require.NoError(t, json.Unmarshal(data, &decoded))

		assert.Equal(t, original.General, decoded.General)
		assert.Equal(t, original.Internal, decoded.Internal)
		assert.Equal(t, original.Reports, decoded.Reports)
		assert.Equal(t, "coord1", decoded.GetCoordinationChannel("peer1"))
	})

	t.Run("existing JSON without coordination_channels deserialises cleanly", func(t *testing.T) {
		legacyJSON := `{"general":"g1","internal":"i1","reports":"r1"}`
		var ids WorkgroupChannelIDs
		require.NoError(t, json.Unmarshal([]byte(legacyJSON), &ids))
		assert.Equal(t, "g1", ids.General)
		assert.Nil(t, ids.CoordinationChannels)
		assert.Equal(t, "", ids.GetCoordinationChannel("any"))
	})
}

func TestWorkgroup_PreSave(t *testing.T) {
	wg := &Workgroup{
		TeamId:      NewId(),
		Name:        "sales",
		DisplayName: "Sales",
		Type:        WorkgroupTypeDepartment,
	}
	wg.PreSave()
	require.NotEmpty(t, wg.Id)
	assert.Greater(t, wg.CreateAt, int64(0))
	assert.Greater(t, wg.UpdateAt, int64(0))
	assert.Equal(t, wg.CreateAt, wg.UpdateAt)
}

func TestWorkgroup_IsValid(t *testing.T) {
	t.Run("valid department workgroup", func(t *testing.T) {
		wg := &Workgroup{Id: NewId(), Name: "sales", Type: WorkgroupTypeDepartment}
		assert.Nil(t, wg.IsValid())
	})

	t.Run("missing ID", func(t *testing.T) {
		wg := &Workgroup{Name: "sales", Type: WorkgroupTypeDepartment}
		assert.NotNil(t, wg.IsValid())
	})

	t.Run("missing name", func(t *testing.T) {
		wg := &Workgroup{Id: NewId(), Type: WorkgroupTypeDepartment}
		assert.NotNil(t, wg.IsValid())
	})

	t.Run("invalid type", func(t *testing.T) {
		wg := &Workgroup{Id: NewId(), Name: "sales", Type: "invalid"}
		assert.NotNil(t, wg.IsValid())
	})

	t.Run("executive type is valid", func(t *testing.T) {
		wg := &Workgroup{Id: NewId(), Name: "exec", Type: WorkgroupTypeExecutive}
		assert.Nil(t, wg.IsValid())
	})
}

func TestDemoSetupRequest(t *testing.T) {
	req := DemoSetupRequest{
		TeamId:          NewId(),
		ObserverUserIds: []string{NewId()},
		LLMServiceId:    "openclaw",
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded DemoSetupRequest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, req.TeamId, decoded.TeamId)
	assert.Equal(t, req.LLMServiceId, decoded.LLMServiceId)
	require.Len(t, decoded.ObserverUserIds, 1)
}

func TestChannelTypeAgentDirect(t *testing.T) {
	assert.Equal(t, ChannelType("A"), ChannelTypeAgentDirect)
	assert.NotEqual(t, ChannelTypeAgentDirect, ChannelTypePrivate)
	assert.NotEqual(t, ChannelTypeAgentDirect, ChannelTypeDirect)
}
