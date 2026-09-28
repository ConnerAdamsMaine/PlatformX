package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/404cnf/meshgrid/internal/core"
)

type Config struct {
	Controller, Token, NodeID, Name string
	Labels                          map[string]string
	Capacity                        core.Resources
	AllowFederation                 bool
	Interval                        time.Duration
	DryRun                          bool
}
type Agent struct {
	cfg     Config
	client  *http.Client
	reports map[string]core.Assignment
}

func New(c Config) *Agent {
	if c.Interval == 0 {
		c.Interval = 10 * time.Second
	}
	return &Agent{cfg: c, client: &http.Client{Timeout: 15 * time.Second}, reports: map[string]core.Assignment{}}
}
func (a *Agent) Run(ctx context.Context) error {
	tick := time.NewTicker(a.cfg.Interval)
	defer tick.Stop()
	for {
		if err := a.once(ctx); err != nil {
			fmt.Printf("agent: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
func (a *Agent) once(ctx context.Context) error {
	reports := make([]core.Assignment, 0, len(a.reports))
	for _, r := range a.reports {
		reports = append(reports, r)
	}
	hb := core.Heartbeat{Node: core.Node{ID: a.cfg.NodeID, Name: a.cfg.Name, OS: runtime.GOOS, Arch: runtime.GOARCH, Labels: a.cfg.Labels, Capacity: a.cfg.Capacity, AllowFederation: a.cfg.AllowFederation}, Assignments: reports}
	b, _ := json.Marshal(hb)
	req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.cfg.Controller, "/")+"/v1/heartbeat", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("controller returned %s", resp.Status)
	}
	var reply core.AgentReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return err
	}
	return a.reconcile(ctx, reply.Assignments)
}
func (a *Agent) reconcile(ctx context.Context, want []core.AssignmentView) error {
	wanted := map[string]core.AssignmentView{}
	for _, v := range want {
		wanted[v.Assignment.ID] = v
	}
	for id, r := range a.reports {
		if _, ok := wanted[id]; !ok || wanted[id].Assignment.State == "deleted" {
			_ = a.remove(ctx, id)
			r.State = "deleted"
			r.UpdatedAt = time.Now().UTC()
			a.reports[id] = r
		}
	}
	ids := make([]string, 0, len(wanted))
	for id := range wanted {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := wanted[id]
		if v.Assignment.State == "deleted" {
			continue
		}
		running := a.running(ctx, id)
		r := v.Assignment
		if !running {
			if err := a.start(ctx, id, v.Workload); err != nil {
				r.State = "failed"
				r.Message = err.Error()
			} else {
				r.State = "running"
				r.Message = ""
			}
		} else {
			r.State = "running"
		}
		r.UpdatedAt = time.Now().UTC()
		a.reports[id] = r
	}
	return nil
}
func (a *Agent) name(id string) string {
	return "meshgrid-" + strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, id)
}
func (a *Agent) running(ctx context.Context, id string) bool {
	if a.cfg.DryRun {
		return a.reports[id].State == "running"
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", a.name(id)).Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
func (a *Agent) remove(ctx context.Context, id string) error {
	if a.cfg.DryRun {
		return nil
	}
	return exec.CommandContext(ctx, "docker", "rm", "-f", a.name(id)).Run()
}
func (a *Agent) start(ctx context.Context, id string, s core.WorkloadSpec) error {
	if a.cfg.DryRun {
		return nil
	}
	args := []string{"run", "-d", "--name", a.name(id), "--label", "meshgrid.assignment=" + id, "--restart", restart(s.Restart)}
	if s.Resources.CPUMillis > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.3f", float64(s.Resources.CPUMillis)/1000))
	}
	if s.Resources.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", s.Resources.MemoryMB))
	}
	if s.Resources.GPUs > 0 {
		args = append(args, "--gpus", fmt.Sprintf("%d", s.Resources.GPUs))
	}
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+s.Env[k])
	}
	args = append(args, s.Image)
	args = append(args, s.Command...)
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
func restart(v string) string {
	switch v {
	case "no", "always", "unless-stopped", "on-failure":
		return v
	default:
		return "unless-stopped"
	}
}
