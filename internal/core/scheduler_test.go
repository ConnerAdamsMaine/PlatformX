package core

import (
	"testing"
	"time"
)

func TestReconcilePlacesReplica(t *testing.T) {
	now := time.Now()
	s := State{Nodes: map[string]Node{"n1": {ID: "n1", Capacity: Resources{CPUMillis: 2000, MemoryMB: 2048}, LastSeen: now}}, Workloads: map[string]Workload{"w1": {ID: "w1", Spec: WorkloadSpec{Name: "web", Image: "nginx", Replicas: 1, Resources: Resources{CPUMillis: 500, MemoryMB: 128}}}}, Assignments: map[string]Assignment{}, Peers: map[string]Peer{}}
	Reconcile(&s, now)
	if got := s.Assignments["w1-0"].NodeID; got != "n1" {
		t.Fatalf("expected n1, got %q", got)
	}
}
func TestSelectorAndCapacity(t *testing.T) {
	now := time.Now()
	s := State{Nodes: map[string]Node{"small": {ID: "small", Labels: map[string]string{"zone": "a"}, Capacity: Resources{MemoryMB: 128}, LastSeen: now}, "large": {ID: "large", Labels: map[string]string{"zone": "b"}, Capacity: Resources{MemoryMB: 4096}, LastSeen: now}}, Workloads: map[string]Workload{"w": {ID: "w", Spec: WorkloadSpec{Name: "x", Image: "x", Replicas: 1, Resources: Resources{MemoryMB: 1024}, NodeSelector: map[string]string{"zone": "b"}}}}, Assignments: map[string]Assignment{}, Peers: map[string]Peer{}}
	Reconcile(&s, now)
	if s.Assignments["w-0"].NodeID != "large" {
		t.Fatal("scheduler ignored selector/capacity")
	}
}
