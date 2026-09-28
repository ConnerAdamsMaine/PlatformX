package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	path string
	s    State
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, s: State{Nodes: map[string]Node{}, Workloads: map[string]Workload{}, Assignments: map[string]Assignment{}, Peers: map[string]Peer{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.s); err != nil {
		return nil, err
	}
	if s.s.Nodes == nil {
		s.s.Nodes = map[string]Node{}
	}
	if s.s.Workloads == nil {
		s.s.Workloads = map[string]Workload{}
	}
	if s.s.Assignments == nil {
		s.s.Assignments = map[string]Assignment{}
	}
	if s.s.Peers == nil {
		s.s.Peers = map[string]Peer{}
	}
	return s, nil
}

func (s *Store) View(fn func(State) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(clone(s.s))
}

func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.s); err != nil {
		return err
	}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clone(v State) State {
	b, _ := json.Marshal(v)
	var out State
	_ = json.Unmarshal(b, &out)
	return out
}
