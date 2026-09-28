package core

import (
	"fmt"
	"sort"
	"time"
)

const nodeTimeout = 45 * time.Second

func Reconcile(s *State, now time.Time) {
	for _, w := range s.Workloads {
		existing := map[int]bool{}
		for _, a := range s.Assignments {
			if a.WorkloadID == w.ID && a.State != "failed" {
				existing[a.Replica] = true
			}
		}
		for replica := 0; replica < w.Spec.Replicas; replica++ {
			if existing[replica] {
				continue
			}
			nodeID := chooseNode(*s, w.Spec, now)
			id := fmt.Sprintf("%s-%d", w.ID, replica)
			state := "pending"
			if nodeID != "" {
				state = "assigned"
			}
			s.Assignments[id] = Assignment{ID: id, WorkloadID: w.ID, Replica: replica, NodeID: nodeID, State: state, UpdatedAt: now}
		}
	}
}

func chooseNode(s State, spec WorkloadSpec, now time.Time) string {
	type candidate struct {
		id   string
		free Resources
	}
	var choices []candidate
	for id, n := range s.Nodes {
		if now.Sub(n.LastSeen) > nodeTimeout || !labelsMatch(n.Labels, spec.NodeSelector) {
			continue
		}
		free := n.Capacity.Sub(n.Allocated)
		for _, a := range s.Assignments {
			if a.NodeID != id || (a.State != "assigned" && a.State != "running") {
				continue
			}
			if w, ok := s.Workloads[a.WorkloadID]; ok {
				free = free.Sub(w.Spec.Resources)
			}
		}
		if free.Fits(spec.Resources) {
			choices = append(choices, candidate{id, free})
		}
	}
	sort.Slice(choices, func(i, j int) bool {
		if choices[i].free.MemoryMB == choices[j].free.MemoryMB {
			return choices[i].id < choices[j].id
		}
		return choices[i].free.MemoryMB > choices[j].free.MemoryMB
	})
	if len(choices) == 0 {
		return ""
	}
	return choices[0].id
}

func labelsMatch(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
