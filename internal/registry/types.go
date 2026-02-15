package registry

import "time"

type AgentRegistration struct {
	AgentID      string    `json:"agentID"`
	VMName       string    `json:"vmName"`
	VMIP         string    `json:"vmIP"`
	Project      string    `json:"project"`
	Tool         string    `json:"tool"`
	Ports        []int     `json:"ports,omitempty"`
	State        string    `json:"state"` // registered, running, completed, failed
	RegisteredAt time.Time `json:"registeredAt"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
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
