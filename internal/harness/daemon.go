package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mateo/agentvm/internal/orchestrator"
)

const (
	taskConfigPath = "/etc/agent-config/task.json"
	workspaceBase  = "/home/lima/workspace"
)

type Daemon struct {
	task     *orchestrator.TaskConfig
	reporter *Reporter
	executor *Executor
}

func NewDaemon() (*Daemon, error) {
	task, err := orchestrator.ReadTaskConfig(taskConfigPath)
	if err != nil {
		return nil, fmt.Errorf("reading task config: %w", err)
	}

	reporter := NewReporter(fmt.Sprintf("http://%s", task.HostAddr))

	return &Daemon{
		task:     task,
		reporter: reporter,
		executor: NewExecutor(),
	}, nil
}

func (d *Daemon) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	log.Printf("Agent harness starting: agent=%s project=%s tool=%s",
		d.task.AgentID, d.task.Project, d.task.Tool)

	// Step 1: Register with host
	d.reporter.Report(d.task.AgentID, "starting", "Harness initializing")

	// Step 2: Setup workspace
	repoDir, err := d.setupWorkspace(ctx)
	if err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Workspace setup failed: %v", err))
		return err
	}

	// Step 3: Create branch
	git := NewGit(repoDir)
	if err := git.CreateBranch(d.task.Branch); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Branch creation failed: %v", err))
		return err
	}

	d.reporter.Report(d.task.AgentID, "executing", fmt.Sprintf("Running %s", d.task.Tool))

	// Step 4: Execute coding tool with constraints
	constrainer := NewConstrainer(d.task.MaxTime)
	result, err := d.executor.Execute(ctx, constrainer, ExecuteConfig{
		Tool:      d.task.Tool,
		Prompt:    d.task.Prompt,
		WorkDir:   repoDir,
		EnvVars:   d.task.EnvVars,
	})
	if err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Execution failed: %v", err))
		return err
	}

	// Step 5: Push results
	d.reporter.Report(d.task.AgentID, "pushing", "Pushing branch")
	if err := git.AddAll(); err != nil {
		log.Printf("Warning: git add failed: %v", err)
	}
	if err := git.Commit(fmt.Sprintf("agent/%s: %s", d.task.AgentID, truncate(d.task.Prompt, 50))); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}
	if err := git.Push(d.task.Branch); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Push failed: %v", err))
		return err
	}

	// Step 6: Report completion
	state := "completed"
	if result.ExitCode != 0 {
		state = "failed"
	}
	d.reporter.Report(d.task.AgentID, state,
		fmt.Sprintf("Exit code: %d, Duration: %s", result.ExitCode, result.Duration))

	// Write result report locally
	d.writeReport(result)

	log.Printf("Agent harness finished: state=%s exit=%d duration=%s",
		state, result.ExitCode, result.Duration)
	return nil
}

func (d *Daemon) setupWorkspace(ctx context.Context) (string, error) {
	d.reporter.Report(d.task.AgentID, "cloning", fmt.Sprintf("Cloning %s", d.task.RepoURL))

	repoDir := filepath.Join(workspaceBase, d.task.Project)
	if err := os.MkdirAll(workspaceBase, 0755); err != nil {
		return "", err
	}

	git := NewGit(workspaceBase)
	if err := git.Clone(d.task.RepoURL, repoDir); err != nil {
		return "", fmt.Errorf("cloning repo: %w", err)
	}

	return repoDir, nil
}

func (d *Daemon) writeReport(result *ExecuteResult) {
	report := map[string]interface{}{
		"agentID":  d.task.AgentID,
		"project":  d.task.Project,
		"tool":     d.task.Tool,
		"exitCode": result.ExitCode,
		"duration": result.Duration.String(),
		"branch":   d.task.Branch,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile("/etc/agent-config/report.json", data, 0644)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
