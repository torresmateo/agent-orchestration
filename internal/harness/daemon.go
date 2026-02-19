package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mateo/agentvm/internal/orchestrator"
)

const (
	taskConfigPath = "/etc/agent-config/task.json"
)

func getWorkspaceBase() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "workspace")
	}
	return "/tmp/workspace"
}

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
	d.reporter.Report(d.task.AgentID, "starting", "Harness initializing", d.task.Branch)

	// Configure git credentials if GITHUB_TOKEN is set
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		home, _ := os.UserHomeDir()
		credFile := filepath.Join(home, ".git-credentials")
		user := os.Getenv("GITHUB_USER")
		if user == "" {
			user = "git"
		}
		os.WriteFile(credFile, []byte(fmt.Sprintf("https://%s:%s@github.com\n", user, token)), 0600)
		git := NewGit("")
		git.run("config", "--global", "credential.helper", "store")
	}

	// Step 2: Setup workspace
	repoDir, err := d.setupWorkspace(ctx)
	if err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Workspace setup failed: %v", err), d.task.Branch)
		return err
	}

	// Step 3: Create branch
	git := NewGit(repoDir)
	if err := git.CreateBranch(d.task.Branch); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Branch creation failed: %v", err), d.task.Branch)
		return err
	}

	d.reporter.Report(d.task.AgentID, "executing", fmt.Sprintf("Running %s", d.task.Tool), d.task.Branch)

	// Step 4: Execute coding tool with constraints
	constrainer := NewConstrainer(d.task.MaxTime)
	result, err := d.executor.Execute(ctx, constrainer, ExecuteConfig{
		Tool:      d.task.Tool,
		Prompt:    d.task.Prompt,
		WorkDir:   repoDir,
		EnvVars:   d.task.EnvVars,
	})
	if err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Execution failed: %v", err), d.task.Branch)
		return err
	}

	// Step 5: Push results
	d.reporter.Report(d.task.AgentID, "pushing", "Pushing branch", d.task.Branch)
	if err := git.AddAll(); err != nil {
		log.Printf("Warning: git add failed: %v", err)
	}
	if err := git.Commit(fmt.Sprintf("agent/%s: %s", d.task.AgentID, truncate(d.task.Prompt, 50))); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}
	if err := git.Push(d.task.Branch); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Push failed: %v", err), d.task.Branch)
		return err
	}

	// Write result report locally
	d.writeReport(result)

	// Step 6: Serve if configured
	if d.task.ServeCommand != "" {
		return d.serve(ctx, repoDir)
	}

	// Step 7: Report completion (non-serve mode)
	state := "completed"
	if result.ExitCode != 0 {
		state = "failed"
	}
	d.reporter.Report(d.task.AgentID, state,
		fmt.Sprintf("Exit code: %d, Duration: %s", result.ExitCode, result.Duration), d.task.Branch)

	log.Printf("Agent harness finished: state=%s exit=%d duration=%s",
		state, result.ExitCode, result.Duration)
	return nil
}

func (d *Daemon) serve(ctx context.Context, repoDir string) error {
	port := d.task.ServePort
	if port <= 0 {
		port = 8080
	}

	log.Printf("Starting serve command: %s (port %d)", d.task.ServeCommand, port)
	d.reporter.Report(d.task.AgentID, "serving", fmt.Sprintf("Starting serve: %s", d.task.ServeCommand), d.task.Branch)

	// Launch serve command via bash -c (supports pipes, &&, etc.)
	cmd := exec.CommandContext(ctx, "bash", "-c", d.task.ServeCommand)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass through environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("AGENT_ID=%s", d.task.AgentID))
	cmd.Env = append(cmd.Env, fmt.Sprintf("AGENT_PROJECT=%s", d.task.Project))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", port))
	for k, v := range d.task.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Serve command failed to start: %v", err), d.task.Branch)
		return fmt.Errorf("starting serve command: %w", err)
	}

	// Wait for port to become ready (up to 5 minutes for docker builds)
	if err := d.waitForPort(ctx, port, 5*time.Minute); err != nil {
		d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Port %d never became ready: %v", port, err), d.task.Branch)
		cmd.Process.Kill()
		return err
	}

	log.Printf("Serving on port %d", port)

	// Get VM IP and register with host
	vmIP, err := d.getVMIP()
	if err != nil {
		log.Printf("Warning: could not determine VM IP: %v", err)
		vmIP = "unknown"
	}

	hostname, _ := os.Hostname()
	if err := d.reporter.Register(
		d.task.AgentID,
		hostname,
		vmIP,
		d.task.Project,
		d.task.Tool,
		[]int{port},
	); err != nil {
		log.Printf("Warning: registration failed: %v", err)
	} else {
		log.Printf("Registered with host: %s -> %s:%d", d.task.AgentID, vmIP, port)
	}

	d.reporter.Report(d.task.AgentID, "serving",
		fmt.Sprintf("Serving on port %d, registered at %s:%d", port, vmIP, port), d.task.Branch)

	// Block until context is cancelled (systemd stop) or serve process exits
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping serve process")
		cmd.Process.Signal(syscall.SIGTERM)
		// Give it 10 seconds to shut down
		select {
		case <-doneCh:
		case <-time.After(10 * time.Second):
			cmd.Process.Kill()
		}
		return nil
	case err := <-doneCh:
		if err != nil {
			d.reporter.Report(d.task.AgentID, "failed", fmt.Sprintf("Serve process exited: %v", err), d.task.Branch)
			return fmt.Errorf("serve process exited: %w", err)
		}
		d.reporter.Report(d.task.AgentID, "completed", "Serve process exited cleanly", d.task.Branch)
		return nil
	}
}

func (d *Daemon) waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	log.Printf("Waiting for port %d to become ready (timeout: %s)...", port, timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			log.Printf("Port %d is ready", port)
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("port %d not ready after %s", port, timeout)
}

func (d *Daemon) getVMIP() (string, error) {
	// Prefer lima0 interface (shared vmnet, routable from host)
	// Fall back to first non-loopback IP from hostname -I
	out, err := exec.Command("ip", "-4", "addr", "show", "lima0").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "inet ") {
				// Format: "inet 192.168.65.34/24 ..."
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					ip := strings.Split(parts[1], "/")[0]
					return ip, nil
				}
			}
		}
	}

	// Fallback: hostname -I
	out, err = exec.Command("hostname", "-I").Output()
	if err != nil {
		return "", fmt.Errorf("could not determine VM IP: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("no IP address found")
	}
	return parts[0], nil
}

func (d *Daemon) setupWorkspace(ctx context.Context) (string, error) {
	d.reporter.Report(d.task.AgentID, "cloning", fmt.Sprintf("Cloning %s", d.task.RepoURL), d.task.Branch)

	wsBase := getWorkspaceBase()
	repoDir := filepath.Join(wsBase, d.task.Project)

	// Clean existing workspace to handle retries and re-dispatches
	if _, err := os.Stat(repoDir); err == nil {
		os.RemoveAll(repoDir)
	}

	if err := os.MkdirAll(wsBase, 0755); err != nil {
		return "", err
	}

	git := NewGit(wsBase)
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
