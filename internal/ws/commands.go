package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/mateo/agentvm/internal/network"
	"github.com/mateo/agentvm/internal/orchestrator"
	"github.com/mateo/agentvm/internal/pool"
	"github.com/mateo/agentvm/internal/registry"
)

// CommandHandler processes commands received via WebSocket.
type CommandHandler struct {
	orch    *orchestrator.Orchestrator
	poolMgr *pool.Manager
	store   *registry.Store
	traefik *network.TraefikWriter
	sshfs   *SSHFSManager
}

// NewCommandHandler creates a new command handler.
func NewCommandHandler(
	orch *orchestrator.Orchestrator,
	poolMgr *pool.Manager,
	store *registry.Store,
	traefik *network.TraefikWriter,
	sshfs *SSHFSManager,
) *CommandHandler {
	return &CommandHandler{
		orch:    orch,
		poolMgr: poolMgr,
		store:   store,
		traefik: traefik,
		sshfs:   sshfs,
	}
}

// Handle dispatches a command to the appropriate handler.
func (ch *CommandHandler) Handle(client *Client, cmd CommandPayload) {
	var result CommandResultPayload
	result.ID = cmd.ID

	switch cmd.Action {
	case "kill":
		result = ch.handleKill(cmd)
	case "dispatch":
		result = ch.handleDispatch(cmd)
	case "mount":
		result = ch.handleMount(cmd)
	case "unmount":
		result = ch.handleUnmount(cmd)
	case "shell":
		result = ch.handleShell(cmd)
	default:
		result.Error = "unknown action: " + cmd.Action
	}

	msg, err := MakeEnvelope(TypeCommandResult, result)
	if err != nil {
		log.Printf("CommandHandler: failed to make envelope: %v", err)
		return
	}
	client.Send(msg)
}

func (ch *CommandHandler) handleKill(cmd CommandPayload) CommandResultPayload {
	var args struct {
		AgentID string `json:"agentID"`
	}
	if err := json.Unmarshal(cmd.Args, &args); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: "invalid args: " + err.Error()}
	}

	slot, ok := ch.poolMgr.GetSlot(args.AgentID)
	if !ok {
		return CommandResultPayload{ID: cmd.ID, Error: "agent not found"}
	}

	if err := ch.poolMgr.Release(slot.Name); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: err.Error()}
	}
	ch.traefik.RemoveRoute(args.AgentID)
	ch.store.Deregister(args.AgentID)

	return CommandResultPayload{ID: cmd.ID, Success: true, Message: "agent killed"}
}

func (ch *CommandHandler) handleDispatch(cmd CommandPayload) CommandResultPayload {
	var args struct {
		Project  string            `json:"project"`
		RepoURL  string            `json:"repoURL"`
		Issue    string            `json:"issue"`
		Tool     string            `json:"tool"`
		Prompt   string            `json:"prompt"`
		Branch   string            `json:"branch"`
		MaxTime  int               `json:"maxTime"`
		EnvVars  map[string]string `json:"envVars"`
	}
	if err := json.Unmarshal(cmd.Args, &args); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: "invalid args: " + err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := ch.orch.Dispatch(ctx, orchestrator.DispatchRequest{
		Project: args.Project,
		RepoURL: args.RepoURL,
		Issue:   args.Issue,
		Tool:    args.Tool,
		Prompt:  args.Prompt,
		Branch:  args.Branch,
		MaxTime: args.MaxTime,
		EnvVars: args.EnvVars,
	})
	if err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: err.Error()}
	}

	return CommandResultPayload{
		ID:      cmd.ID,
		Success: true,
		Message: "dispatched " + result.AgentID + " to " + result.VMName,
	}
}

func (ch *CommandHandler) handleMount(cmd CommandPayload) CommandResultPayload {
	var args struct {
		AgentID   string `json:"agentID"`
		MountPath string `json:"mountPath"`
	}
	if err := json.Unmarshal(cmd.Args, &args); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: "invalid args: " + err.Error()}
	}

	slot, ok := ch.poolMgr.GetSlot(args.AgentID)
	if !ok {
		return CommandResultPayload{ID: cmd.ID, Error: "agent not found"}
	}

	mountPoint, err := ch.sshfs.Mount(slot.Name, args.AgentID, slot.Project, args.MountPath)
	if err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: err.Error()}
	}

	return CommandResultPayload{ID: cmd.ID, Success: true, Message: "mounted at " + mountPoint}
}

func (ch *CommandHandler) handleUnmount(cmd CommandPayload) CommandResultPayload {
	var args struct {
		AgentID string `json:"agentID"`
	}
	if err := json.Unmarshal(cmd.Args, &args); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: "invalid args: " + err.Error()}
	}

	if err := ch.sshfs.Unmount(args.AgentID); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: err.Error()}
	}

	return CommandResultPayload{ID: cmd.ID, Success: true, Message: "unmounted"}
}

func (ch *CommandHandler) handleShell(cmd CommandPayload) CommandResultPayload {
	var args struct {
		AgentID string `json:"agentID"`
	}
	if err := json.Unmarshal(cmd.Args, &args); err != nil {
		return CommandResultPayload{ID: cmd.ID, Error: "invalid args: " + err.Error()}
	}

	slot, ok := ch.poolMgr.GetSlot(args.AgentID)
	if !ok {
		return CommandResultPayload{ID: cmd.ID, Error: "agent not found"}
	}

	// Shell is launched client-side via `open -a Terminal limactl shell <vm>`
	// We just return the VM name for the client to use
	return CommandResultPayload{
		ID:      cmd.ID,
		Success: true,
		Message: slot.Name,
	}
}
