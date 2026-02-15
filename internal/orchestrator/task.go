package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type TaskConfig struct {
	AgentID    string            `json:"agentID"`
	Project    string            `json:"project"`
	RepoURL    string            `json:"repoURL"`
	Issue      string            `json:"issue,omitempty"`
	Tool       string            `json:"tool"`
	Prompt     string            `json:"prompt"`
	Branch     string            `json:"branch"`
	MaxTime    int               `json:"maxTime"` // minutes
	MaxTokens  int               `json:"maxTokens,omitempty"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
	HostAddr   string            `json:"hostAddr"`   // e.g. "host.lima.internal:8090"
	DispatchedAt time.Time       `json:"dispatchedAt"`
}

var validTools = map[string]bool{
	"claude-code": true,
	"opencode":    true,
	"amp":         true,
	"cline":       true,
}

func ValidateTask(tc *TaskConfig) error {
	if tc.Project == "" {
		return fmt.Errorf("project is required")
	}
	if tc.RepoURL == "" {
		return fmt.Errorf("repoURL is required")
	}
	if tc.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if !validTools[tc.Tool] {
		return fmt.Errorf("invalid tool %q (valid: claude-code, opencode, amp, cline)", tc.Tool)
	}
	if tc.MaxTime <= 0 {
		tc.MaxTime = 30
	}
	if tc.Branch == "" {
		tc.Branch = fmt.Sprintf("agent/%s/%s", tc.Project, tc.AgentID)
	}
	return nil
}

func WriteTaskConfig(tc *TaskConfig, path string) error {
	data, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling task config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func ReadTaskConfig(path string) (*TaskConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading task config: %w", err)
	}
	var tc TaskConfig
	if err := json.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("parsing task config: %w", err)
	}
	return &tc, nil
}
