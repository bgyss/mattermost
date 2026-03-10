// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentDef(id string, caps ...string) *model.AgentDefinition {
	return &model.AgentDefinition{
		Id:           id,
		DisplayName:  id,
		Capabilities: caps,
	}
}

func TestCapabilityRouter_RegisterAndRoute(t *testing.T) {
	r := NewCapabilityRouter()

	r.Register(agentDef("agent1", "code_review", "python"))
	r.Register(agentDef("agent2", "code_review", "golang"))

	def, err := r.Route([]string{"code_review", "python"})
	require.NoError(t, err)
	assert.Equal(t, "agent1", def.Id)
}

func TestCapabilityRouter_NoMatch(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "code_review"))

	_, err := r.Route([]string{"data_analysis"})
	require.Error(t, err)
}

func TestCapabilityRouter_EmptyCapabilities_ReturnsAnyAgent(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "code_review"))

	// Routing with no required capabilities should succeed if any agent exists.
	_, err := r.Route([]string{})
	require.NoError(t, err)
}

func TestCapabilityRouter_EmptyRouter(t *testing.T) {
	r := NewCapabilityRouter()
	_, err := r.Route([]string{"anything"})
	require.Error(t, err)
}

func TestCapabilityRouter_LeastLoaded(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "search"))
	r.Register(agentDef("agent2", "search"))

	// Increment load on agent1
	r.IncrLoad("agent1")
	r.IncrLoad("agent1")

	// agent2 should be chosen as least loaded
	def, err := r.Route([]string{"search"})
	require.NoError(t, err)
	assert.Equal(t, "agent2", def.Id)
}

func TestCapabilityRouter_LeastLoadedAfterDecrement(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "search"))
	r.Register(agentDef("agent2", "search"))

	r.IncrLoad("agent1")
	r.IncrLoad("agent2")
	r.IncrLoad("agent2")
	r.DecrLoad("agent2") // agent2 back to 1

	// Both at 1 now — either is fine; just verify no error.
	_, err := r.Route([]string{"search"})
	require.NoError(t, err)
}

func TestCapabilityRouter_UnregisterRemovesAgent(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "ocr"))
	r.Unregister("agent1")

	_, err := r.Route([]string{"ocr"})
	require.Error(t, err)
}

func TestCapabilityRouter_MultipleCapabilitiesRequired(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("specialist", "vision", "ocr", "pdf"))
	r.Register(agentDef("generalist", "vision"))

	// Only specialist has all three.
	def, err := r.Route([]string{"vision", "ocr", "pdf"})
	require.NoError(t, err)
	assert.Equal(t, "specialist", def.Id)
}

func TestCapabilityRouter_RegisterOverwrite(t *testing.T) {
	r := NewCapabilityRouter()
	r.Register(agentDef("agent1", "old_cap"))
	// Re-register with different capabilities.
	r.Register(agentDef("agent1", "new_cap"))

	_, err := r.Route([]string{"old_cap"})
	require.Error(t, err, "old capability should be removed after re-register")

	def, err := r.Route([]string{"new_cap"})
	require.NoError(t, err)
	assert.Equal(t, "agent1", def.Id)
}
