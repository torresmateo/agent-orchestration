package pool

import "time"

type SlotState string

const (
	SlotIdle     SlotState = "idle"
	SlotActive   SlotState = "active"
	SlotCold     SlotState = "cold"
	SlotCreating SlotState = "creating"
)

type VMSlot struct {
	Name      string    `json:"name"`
	State     SlotState `json:"state"`
	AgentID   string    `json:"agentID,omitempty"`
	Project   string    `json:"project,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Issue     string    `json:"issue,omitempty"`
	VMIP      string    `json:"vmIP,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ClaimedAt time.Time `json:"claimedAt,omitempty"`
}

type PoolConfig struct {
	WarmSize   int    `json:"warmSize"`
	MaxVMs     int    `json:"maxVMs"`
	MasterName string `json:"masterName"`
}
