package core

import "time"

type Resources struct {
	CPUMillis int64 `json:"cpuMillis"`
	MemoryMB  int64 `json:"memoryMB"`
	GPUs      int   `json:"gpus"`
}

func (r Resources) Fits(need Resources) bool {
	return r.CPUMillis >= need.CPUMillis && r.MemoryMB >= need.MemoryMB && r.GPUs >= need.GPUs
}

func (r Resources) Sub(v Resources) Resources {
	return Resources{r.CPUMillis - v.CPUMillis, r.MemoryMB - v.MemoryMB, r.GPUs - v.GPUs}
}

type Node struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	OS              string            `json:"os"`
	Arch            string            `json:"arch"`
	Labels          map[string]string `json:"labels,omitempty"`
	Capacity        Resources         `json:"capacity"`
	Allocated       Resources         `json:"allocated"`
	AllowFederation bool              `json:"allowFederation"`
	LastSeen        time.Time         `json:"lastSeen"`
}

type WorkloadSpec struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Replicas     int               `json:"replicas"`
	Resources    Resources         `json:"resources"`
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	Federated    bool              `json:"federated"`
	Restart      string            `json:"restart,omitempty"`
}

type Workload struct {
	ID        string       `json:"id"`
	Owner     string       `json:"owner"`
	Spec      WorkloadSpec `json:"spec"`
	CreatedAt time.Time    `json:"createdAt"`
}

type Assignment struct {
	ID         string    `json:"id"`
	WorkloadID string    `json:"workloadId"`
	Replica    int       `json:"replica"`
	NodeID     string    `json:"nodeId"`
	PeerID     string    `json:"peerId,omitempty"`
	State      string    `json:"state"`
	Message    string    `json:"message,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Heartbeat struct {
	Node        Node         `json:"node"`
	Assignments []Assignment `json:"assignments,omitempty"`
}

type AgentReply struct {
	Assignments []AssignmentView `json:"assignments"`
}

type AssignmentView struct {
	Assignment Assignment   `json:"assignment"`
	Workload   WorkloadSpec `json:"workload"`
}

type Peer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	PublicKey string    `json:"publicKey"`
	Enabled   bool      `json:"enabled"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
	Capacity  Resources `json:"capacity,omitempty"`
}

type State struct {
	Nodes       map[string]Node       `json:"nodes"`
	Workloads   map[string]Workload   `json:"workloads"`
	Assignments map[string]Assignment `json:"assignments"`
	Peers       map[string]Peer       `json:"peers"`
}
