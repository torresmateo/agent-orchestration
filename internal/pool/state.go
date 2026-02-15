package pool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type PersistentState struct {
	Slots   []VMSlot `json:"slots"`
	Counter int      `json:"counter"` // monotonic counter for VM naming
}

type stateStore struct {
	path string
	mu   sync.Mutex
}

func newStateStore(baseDir string) *stateStore {
	return &stateStore{
		path: filepath.Join(baseDir, "pool-state.json"),
	}
}

func (s *stateStore) Load() (PersistentState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state PersistentState
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("reading pool state: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parsing pool state: %w", err)
	}
	return state, nil
}

func (s *stateStore) Save(state PersistentState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
