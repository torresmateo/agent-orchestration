package api

import "time"

// DispatchRequest is sent from agentctl to agentd to start a new agent task.
type DispatchRequest struct {
	Project      string            `json:"project"`
	RepoURL      string            `json:"repoURL"`
	Issue        string            `json:"issue,omitempty"`
	Tool         string            `json:"tool"`       // claude-code, opencode, amp, cline
	Prompt       string            `json:"prompt"`
	Branch       string            `json:"branch,omitempty"`
	MaxTime      int               `json:"maxTime,omitempty"`    // minutes
	MaxTokens    int               `json:"maxTokens,omitempty"`
	EnvVars      map[string]string `json:"envVars,omitempty"`
	ServeCommand string            `json:"serveCommand,omitempty"`
	ServePort    int               `json:"servePort,omitempty"`
}

// DispatchResponse is returned after a successful dispatch.
type DispatchResponse struct {
	AgentID  string `json:"agentID"`
	VMName   string `json:"vmName"`
	VMIP     string `json:"vmIP"`
	Subdomain string `json:"subdomain"`
}

// AgentStatus represents the current state of an agent.
type AgentStatus struct {
	AgentID   string        `json:"agentID"`
	VMName    string        `json:"vmName"`
	VMIP      string        `json:"vmIP"`
	Project   string        `json:"project"`
	Tool      string        `json:"tool"`
	Branch    string        `json:"branch,omitempty"`
	Issue     string        `json:"issue,omitempty"`
	State     string        `json:"state"` // running, completed, failed, killed
	StartedAt time.Time     `json:"startedAt"`
	Elapsed   time.Duration `json:"elapsed"`
	Subdomain string        `json:"subdomain"`
}

// PoolStatus reports pool state.
type PoolStatus struct {
	Warm   int         `json:"warm"`
	Active int         `json:"active"`
	Cold   int         `json:"cold"`
	Agents []AgentStatus `json:"agents,omitempty"`
}

// HarnessStatusReport is sent from agent-harness to agentd.
type HarnessStatusReport struct {
	AgentID    string `json:"agentID"`
	VMName     string `json:"vmName"`
	State      string `json:"state"` // starting, cloning, executing, pushing, completed, failed
	Message    string `json:"message,omitempty"`
	Branch     string `json:"branch,omitempty"`
	ExitCode   int    `json:"exitCode,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
