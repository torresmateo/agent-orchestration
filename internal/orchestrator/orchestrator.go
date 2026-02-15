package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mateo/agentvm/internal/lima"
	"github.com/mateo/agentvm/internal/pool"
)

type Orchestrator struct {
	pool       *pool.Manager
	limaClient lima.Client
	baseDir    string
	hostAddr   string // e.g. "host.lima.internal:8090"
}

func New(pm *pool.Manager, lc lima.Client, baseDir, hostAddr string) *Orchestrator {
	return &Orchestrator{
		pool:       pm,
		limaClient: lc,
		baseDir:    baseDir,
		hostAddr:   hostAddr,
	}
}

type DispatchResult struct {
	AgentID string
	VMName  string
	VMIP    string
}

func (o *Orchestrator) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano()%100000)

	task := &TaskConfig{
		AgentID:      agentID,
		Project:      req.Project,
		RepoURL:      req.RepoURL,
		Issue:        req.Issue,
		Tool:         req.Tool,
		Prompt:       req.Prompt,
		Branch:       req.Branch,
		MaxTime:      req.MaxTime,
		MaxTokens:    req.MaxTokens,
		EnvVars:      req.EnvVars,
		HostAddr:     o.hostAddr,
		DispatchedAt: time.Now(),
	}

	if err := ValidateTask(task); err != nil {
		return nil, fmt.Errorf("invalid task: %w", err)
	}

	// Claim a warm VM
	slot, err := o.pool.Claim(ctx, agentID, req.Project)
	if err != nil {
		return nil, fmt.Errorf("claiming VM: %w", err)
	}

	log.Printf("Dispatching %s to VM %s", agentID, slot.Name)

	// Write task.json to temp file, then copy into VM
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("task-%s.json", agentID))
	if err := WriteTaskConfig(task, tmpFile); err != nil {
		o.pool.Release(slot.Name)
		return nil, fmt.Errorf("writing task config: %w", err)
	}
	defer os.Remove(tmpFile)

	// Copy task.json into VM
	err = o.limaClient.Copy(ctx, lima.CopyOptions{
		Instance:  slot.Name,
		Direction: lima.CopyToVM,
		LocalPath: tmpFile,
		VMPath:    "/etc/agent-config/task.json",
	})
	if err != nil {
		o.pool.Release(slot.Name)
		return nil, fmt.Errorf("injecting task config: %w", err)
	}

	// Write env vars into VM
	envContent := fmt.Sprintf("AGENT_ID=%s\nAGENT_PROJECT=%s\nAGENT_HOST=%s\n",
		agentID, req.Project, o.hostAddr)
	for k, v := range req.EnvVars {
		envContent += fmt.Sprintf("%s=%s\n", k, v)
	}

	envTmp := filepath.Join(os.TempDir(), fmt.Sprintf("env-%s", agentID))
	if err := os.WriteFile(envTmp, []byte(envContent), 0644); err != nil {
		o.pool.Release(slot.Name)
		return nil, fmt.Errorf("writing env file: %w", err)
	}
	defer os.Remove(envTmp)

	err = o.limaClient.Copy(ctx, lima.CopyOptions{
		Instance:  slot.Name,
		Direction: lima.CopyToVM,
		LocalPath: envTmp,
		VMPath:    "/etc/agent-config/env",
	})
	if err != nil {
		o.pool.Release(slot.Name)
		return nil, fmt.Errorf("injecting env config: %w", err)
	}

	// Restart the harness service
	_, err = o.limaClient.Shell(ctx, lima.ShellOptions{
		Instance: slot.Name,
		Command:  "sudo",
		Args:     []string{"systemctl", "restart", "agent-harness.service"},
		Timeout:  30 * time.Second,
	})
	if err != nil {
		o.pool.Release(slot.Name)
		return nil, fmt.Errorf("restarting harness: %w", err)
	}

	return &DispatchResult{
		AgentID: agentID,
		VMName:  slot.Name,
		VMIP:    slot.VMIP,
	}, nil
}

type DispatchRequest struct {
	Project   string
	RepoURL   string
	Issue     string
	Tool      string
	Prompt    string
	Branch    string
	MaxTime   int
	MaxTokens int
	EnvVars   map[string]string
}
