// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package agentruntime

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/mattermost/mattermost/server/public/model"
)

// capabilityEntry tracks an agent's capabilities and current load.
type capabilityEntry struct {
	def         *model.AgentDefinition
	activeCount int64 // atomic — number of tasks currently running
}

// CapabilityRouter routes tasks to the least-loaded agent that has a required capability.
type CapabilityRouter struct {
	mu    sync.RWMutex
	byId  map[string]*capabilityEntry   // agentId → entry
	byCap map[string][]*capabilityEntry // capability → entries that have it
}

// NewCapabilityRouter creates an empty router.
func NewCapabilityRouter() *CapabilityRouter {
	return &CapabilityRouter{
		byId:  make(map[string]*capabilityEntry),
		byCap: make(map[string][]*capabilityEntry),
	}
}

// Register adds or replaces an agent definition in the router.
func (r *CapabilityRouter) Register(def *model.AgentDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove stale capability mappings for this agent if re-registering
	if old, exists := r.byId[def.Id]; exists {
		for _, cap := range old.def.Capabilities {
			r.removeCap(cap, old)
		}
	}

	entry := &capabilityEntry{def: def}
	r.byId[def.Id] = entry
	for _, cap := range def.Capabilities {
		r.byCap[cap] = append(r.byCap[cap], entry)
	}
}

// Unregister removes an agent from the router.
func (r *CapabilityRouter) Unregister(agentId string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.byId[agentId]
	if !exists {
		return
	}
	for _, cap := range entry.def.Capabilities {
		r.removeCap(cap, entry)
	}
	delete(r.byId, agentId)
}

// Route returns the least-loaded agent definition that has all required capabilities.
// Returns an error if no qualifying agent is registered.
func (r *CapabilityRouter) Route(capabilities []string) (*model.AgentDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(capabilities) == 0 {
		// No capability filter — pick least-loaded among all agents
		return r.leastLoaded(r.allEntries()), nil
	}

	// Intersect: find entries that have ALL required capabilities
	var candidates []*capabilityEntry
	for i, cap := range capabilities {
		entries := r.byCap[cap]
		if i == 0 {
			candidates = make([]*capabilityEntry, len(entries))
			copy(candidates, entries)
			continue
		}
		candidates = intersect(candidates, entries)
		if len(candidates) == 0 {
			break
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no agent found with capabilities: %v", capabilities)
	}

	def := r.leastLoaded(candidates)
	if def == nil {
		return nil, fmt.Errorf("no available agent for capabilities: %v", capabilities)
	}
	return def, nil
}

// IncrLoad increments the in-flight task counter for an agent (call before executing).
func (r *CapabilityRouter) IncrLoad(agentId string) {
	r.mu.RLock()
	entry := r.byId[agentId]
	r.mu.RUnlock()
	if entry != nil {
		atomic.AddInt64(&entry.activeCount, 1)
	}
}

// DecrLoad decrements the in-flight task counter (call after task completes).
func (r *CapabilityRouter) DecrLoad(agentId string) {
	r.mu.RLock()
	entry := r.byId[agentId]
	r.mu.RUnlock()
	if entry != nil {
		atomic.AddInt64(&entry.activeCount, -1)
	}
}

// --- internal helpers ---

func (r *CapabilityRouter) removeCap(cap string, target *capabilityEntry) {
	entries := r.byCap[cap]
	filtered := entries[:0]
	for _, e := range entries {
		if e != target {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		delete(r.byCap, cap)
	} else {
		r.byCap[cap] = filtered
	}
}

func (r *CapabilityRouter) allEntries() []*capabilityEntry {
	all := make([]*capabilityEntry, 0, len(r.byId))
	for _, e := range r.byId {
		all = append(all, e)
	}
	return all
}

func (r *CapabilityRouter) leastLoaded(entries []*capabilityEntry) *model.AgentDefinition {
	if len(entries) == 0 {
		return nil
	}
	best := entries[0]
	bestLoad := atomic.LoadInt64(&best.activeCount)
	for _, e := range entries[1:] {
		if load := atomic.LoadInt64(&e.activeCount); load < bestLoad {
			best = e
			bestLoad = load
		}
	}
	return best.def
}

// intersect returns the entries present in both slices.
func intersect(a, b []*capabilityEntry) []*capabilityEntry {
	set := make(map[*capabilityEntry]struct{}, len(b))
	for _, e := range b {
		set[e] = struct{}{}
	}
	result := a[:0]
	for _, e := range a {
		if _, ok := set[e]; ok {
			result = append(result, e)
		}
	}
	return result
}
