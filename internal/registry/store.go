package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu     sync.RWMutex
	agents map[string]*AgentRegistration
	path   string
}

func NewStore(baseDir string) (*Store, error) {
	s := &Store{
		agents: make(map[string]*AgentRegistration),
		path:   filepath.Join(baseDir, "registry.json"),
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Register(reg *AgentRegistration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[reg.AgentID] = reg
	s.persist()
}

func (s *Store) Deregister(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, agentID)
	s.persist()
}

func (s *Store) Get(agentID string) (*AgentRegistration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reg, ok := s.agents[agentID]
	return reg, ok
}

func (s *Store) List() []*AgentRegistration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AgentRegistration, 0, len(s.agents))
	for _, reg := range s.agents {
		result = append(result, reg)
	}
	return result
}

func (s *Store) UpdateState(agentID, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reg, ok := s.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %q not registered", agentID)
	}
	reg.State = state
	s.persist()
	return nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var agents map[string]*AgentRegistration
	if err := json.Unmarshal(data, &agents); err != nil {
		return err
	}
	s.agents = agents
	return nil
}

func (s *Store) persist() {
	data, err := json.MarshalIndent(s.agents, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(s.path), 0755)
	os.WriteFile(s.path, data, 0644)
}
