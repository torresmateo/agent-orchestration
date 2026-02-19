package registry

import "time"

type AgentRegistration struct {
	AgentID       string    `json:"agentID"`
	VMName        string    `json:"vmName"`
	VMIP          string    `json:"vmIP"`
	Project       string    `json:"project"`
	Tool          string    `json:"tool"`
	Branch        string    `json:"branch,omitempty"`
	Message       string    `json:"message,omitempty"`
	Ports         []int     `json:"ports,omitempty"`
	State         string    `json:"state"` // registered, running, completed, failed
	RegisteredAt  time.Time `json:"registeredAt"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
}

// StoreEventType identifies the kind of store change.
type StoreEventType string

const (
	EventAgentRegistered   StoreEventType = "agent.registered"
	EventAgentDeregistered StoreEventType = "agent.deregistered"
	EventAgentUpdated      StoreEventType = "agent.updated"
)

// StoreEvent is emitted when the registry changes.
type StoreEvent struct {
	Type    StoreEventType     `json:"type"`
	AgentID string             `json:"agentID"`
	Agent   *AgentRegistration `json:"agent,omitempty"`
}

type RegisterRequest struct {
	AgentID string `json:"agentID"`
	VMName  string `json:"vmName"`
	VMIP    string `json:"vmIP"`
	Project string `json:"project"`
	Tool    string `json:"tool"`
	Ports   []int  `json:"ports,omitempty"`
}

type RegisterResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type DeregisterRequest struct {
	AgentID string `json:"agentID"`
}
