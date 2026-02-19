package ws

import (
	"encoding/json"
	"time"
)

// Envelope is the top-level WebSocket message format.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// --- Client -> Server messages ---

// SubscribePayload requests subscription to a channel.
type SubscribePayload struct {
	Channel string `json:"channel"` // "status", "logs:<agentId>"
}

// UnsubscribePayload cancels a subscription.
type UnsubscribePayload struct {
	Channel string `json:"channel"`
}

// CommandPayload sends a command to the server.
type CommandPayload struct {
	ID     string          `json:"id"`     // client-generated correlation ID
	Action string          `json:"action"` // kill, dispatch, mount, unmount, shell
	Args   json.RawMessage `json:"args"`
}

// --- Server -> Client messages ---

// AgentSnapshot represents the full state of a single agent.
type AgentSnapshot struct {
	AgentID   string    `json:"agentID"`
	VMName    string    `json:"vmName"`
	VMIP      string    `json:"vmIP"`
	Project   string    `json:"project"`
	Tool      string    `json:"tool"`
	Branch    string    `json:"branch,omitempty"`
	Issue     string    `json:"issue,omitempty"`
	State     string    `json:"state"`
	Message   string    `json:"message,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	Elapsed   string    `json:"elapsed"`
	Subdomain string    `json:"subdomain,omitempty"`
}

// StatusSnapshotPayload is the full state sent on subscribe and periodically.
type StatusSnapshotPayload struct {
	Pool   PoolSnapshot    `json:"pool"`
	Agents []AgentSnapshot `json:"agents"`
}

// PoolSnapshot contains pool-level metrics.
type PoolSnapshot struct {
	Warm   int `json:"warm"`
	Active int `json:"active"`
	Cold   int `json:"cold"`
}

// StatusUpdatePayload is a single agent state change.
type StatusUpdatePayload struct {
	AgentID string `json:"agentID"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

// AgentEventPayload is sent for register/deregister events.
type AgentEventPayload struct {
	AgentID string         `json:"agentID"`
	Agent   *AgentSnapshot `json:"agent,omitempty"`
}

// LogDataPayload is a streamed log line.
type LogDataPayload struct {
	AgentID string `json:"agentID"`
	Line    string `json:"line"`
}

// CommandResultPayload is the response to a command.
type CommandResultPayload struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Message type constants.
const (
	// Client -> Server
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypeCommand     = "command"

	// Server -> Client
	TypeStatusSnapshot    = "status.snapshot"
	TypeStatusUpdate      = "status.update"
	TypeAgentRegistered   = "agent.registered"
	TypeAgentDeregistered = "agent.deregistered"
	TypeLogsData          = "logs.data"
	TypeCommandResult     = "command.result"
)

// Channel name constants.
const (
	ChannelStatus = "status"
	// logs channels are "logs:<agentId>"
)

// MakeEnvelope creates an Envelope with the given type and payload.
func MakeEnvelope(msgType string, payload interface{}) ([]byte, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Payload: p})
}
