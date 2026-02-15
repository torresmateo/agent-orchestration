package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTask_Valid(t *testing.T) {
	tc := &TaskConfig{
		AgentID: "agent-1",
		Project: "myproject",
		RepoURL: "https://github.com/user/repo",
		Tool:    "claude-code",
		Prompt:  "Fix the login bug",
	}

	if err := ValidateTask(tc); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}

	if tc.MaxTime != 30 {
		t.Errorf("expected default maxTime 30, got %d", tc.MaxTime)
	}
	if tc.Branch == "" {
		t.Error("expected branch to be auto-generated")
	}
}

func TestValidateTask_MissingProject(t *testing.T) {
	tc := &TaskConfig{
		RepoURL: "https://github.com/user/repo",
		Tool:    "claude-code",
		Prompt:  "Fix bug",
	}
	if err := ValidateTask(tc); err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestValidateTask_MissingRepo(t *testing.T) {
	tc := &TaskConfig{
		Project: "myproject",
		Tool:    "claude-code",
		Prompt:  "Fix bug",
	}
	if err := ValidateTask(tc); err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestValidateTask_MissingPrompt(t *testing.T) {
	tc := &TaskConfig{
		Project: "myproject",
		RepoURL: "https://github.com/user/repo",
		Tool:    "claude-code",
	}
	if err := ValidateTask(tc); err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestValidateTask_InvalidTool(t *testing.T) {
	tc := &TaskConfig{
		Project: "myproject",
		RepoURL: "https://github.com/user/repo",
		Tool:    "invalid-tool",
		Prompt:  "Fix bug",
	}
	if err := ValidateTask(tc); err == nil {
		t.Fatal("expected error for invalid tool")
	}
}

func TestValidateTask_AllTools(t *testing.T) {
	tools := []string{"claude-code", "opencode", "amp", "cline"}
	for _, tool := range tools {
		tc := &TaskConfig{
			AgentID: "agent-1",
			Project: "proj",
			RepoURL: "https://github.com/user/repo",
			Tool:    tool,
			Prompt:  "Do something",
		}
		if err := ValidateTask(tc); err != nil {
			t.Errorf("tool %q should be valid, got: %v", tool, err)
		}
	}
}

func TestWriteAndReadTaskConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "task.json")

	tc := &TaskConfig{
		AgentID: "agent-1",
		Project: "myproject",
		RepoURL: "https://github.com/user/repo",
		Tool:    "claude-code",
		Prompt:  "Fix the login bug",
		MaxTime: 30,
		Branch:  "agent/myproject/agent-1",
		EnvVars: map[string]string{"FOO": "bar"},
	}

	if err := WriteTaskConfig(tc, path); err != nil {
		t.Fatalf("WriteTaskConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("task file not found: %v", err)
	}

	loaded, err := ReadTaskConfig(path)
	if err != nil {
		t.Fatalf("ReadTaskConfig failed: %v", err)
	}

	if loaded.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", loaded.AgentID)
	}
	if loaded.Project != "myproject" {
		t.Errorf("expected myproject, got %s", loaded.Project)
	}
	if loaded.Tool != "claude-code" {
		t.Errorf("expected claude-code, got %s", loaded.Tool)
	}
	if loaded.EnvVars["FOO"] != "bar" {
		t.Errorf("expected env var FOO=bar, got %s", loaded.EnvVars["FOO"])
	}
}
