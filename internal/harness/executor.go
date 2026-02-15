package harness

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type ExecuteConfig struct {
	Tool    string
	Prompt  string
	WorkDir string
	EnvVars map[string]string
}

type ExecuteResult struct {
	ExitCode int
	Duration time.Duration
	Output   string
}

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, c *Constrainer, cfg ExecuteConfig) (*ExecuteResult, error) {
	args := buildCommand(cfg.Tool, cfg.Prompt)
	if len(args) == 0 {
		return nil, fmt.Errorf("unsupported tool: %s", cfg.Tool)
	}

	log.Printf("Executing: %v in %s", args, cfg.WorkDir)

	// Apply time constraint
	execCtx := c.WithContext(ctx)

	cmd := exec.CommandContext(execCtx, args[0], args[1:]...)
	cmd.Dir = cfg.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range cfg.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("execution error: %w", err)
		}
	}

	return &ExecuteResult{
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

func buildCommand(tool, prompt string) []string {
	switch tool {
	case "claude-code":
		return []string{"claude", "--dangerously-skip-permissions", "-p", prompt}
	case "opencode":
		return []string{"opencode", "run", "--prompt", prompt}
	case "amp":
		return []string{"amp", "run", prompt}
	case "cline":
		return []string{"cline", "--task", prompt}
	default:
		return nil
	}
}
