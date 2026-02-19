package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu          sync.RWMutex
	agents      map[string]*AgentRegistration
	path        string
	subscribers []chan StoreEvent
	subMu       sync.Mutex
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

// Subscribe returns a channel that receives store events.
// The caller should eventually call Unsubscribe to clean up.
func (s *Store) Subscribe() chan StoreEvent {
	ch := make(chan StoreEvent, 64)
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subMu.Unlock()
	return ch
}

// Unsubscribe removes a previously subscribed channel.
func (s *Store) Unsubscribe(ch chan StoreEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

func (s *Store) notify(event StoreEvent) {
	s.subMu.Lock()
	subs := make([]chan StoreEvent, len(s.subscribers))
	copy(subs, s.subscribers)
	s.subMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Drop event if subscriber is slow
		}
	}
}

func (s *Store) Register(reg *AgentRegistration) {
	s.mu.Lock()
	s.agents[reg.AgentID] = reg
	s.persist()
	s.mu.Unlock()

	s.notify(StoreEvent{
		Type:    EventAgentRegistered,
		AgentID: reg.AgentID,
		Agent:   reg,
	})
}

func (s *Store) Deregister(agentID string) {
	s.mu.Lock()
	agent := s.agents[agentID]
	delete(s.agents, agentID)
	s.persist()
	s.mu.Unlock()

	s.notify(StoreEvent{
		Type:    EventAgentDeregistered,
		AgentID: agentID,
		Agent:   agent,
	})
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

func (s *Store) UpdateState(agentID, state, message, branch string) error {
	s.mu.Lock()
	reg, ok := s.agents[agentID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("agent %q not registered", agentID)
	}
	reg.State = state
	reg.LastHeartbeat = time.Now()
	if message != "" {
		reg.Message = message
	}
	if branch != "" {
		reg.Branch = branch
	}
	s.persist()
	s.mu.Unlock()

	s.notify(StoreEvent{
		Type:    EventAgentUpdated,
		AgentID: agentID,
		Agent:   reg,
	})
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
