package lima

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.yaml")

	cfg := TemplateConfig{
		CPUs:         4,
		MemoryGiB:    8,
		DiskGiB:      50,
		SharedDir:    "/home/user/.agentvm/shared",
		HostIP:       "host.lima.internal",
		RegistryPort: 8090,
	}

	if err := RenderTemplate(cfg, outputPath); err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	s := string(content)

	if !strings.Contains(s, "cpus: 4") {
		t.Error("expected cpus: 4")
	}
	if !strings.Contains(s, `memory: "8GiB"`) {
		t.Error("expected memory: 8GiB")
	}
	if !strings.Contains(s, `disk: "50GiB"`) {
		t.Error("expected disk: 50GiB")
	}
	if !strings.Contains(s, "vmType: vz") {
		t.Error("expected vmType: vz")
	}
	if !strings.Contains(s, "vzNAT: true") {
		t.Error("expected vzNAT networking")
	}
	if !strings.Contains(s, "/home/user/.agentvm/shared") {
		t.Error("expected shared dir path")
	}
	if !strings.Contains(s, "host.lima.internal") {
		t.Error("expected host IP")
	}
	if !strings.Contains(s, "docker-ce") {
		t.Error("expected Docker CE installation")
	}
	if !strings.Contains(s, "agent-harness") {
		t.Error("expected agent-harness setup")
	}
}

func TestDefaultTemplateConfig(t *testing.T) {
	cfg := DefaultTemplateConfig()

	if cfg.CPUs != 2 {
		t.Errorf("expected 2 CPUs, got %d", cfg.CPUs)
	}
	if cfg.MemoryGiB != 3 {
		t.Errorf("expected 3 GiB, got %d", cfg.MemoryGiB)
	}
	if cfg.DiskGiB != 30 {
		t.Errorf("expected 30 GiB, got %d", cfg.DiskGiB)
	}
	if cfg.RegistryPort != 8090 {
		t.Errorf("expected port 8090, got %d", cfg.RegistryPort)
	}
	if !strings.Contains(cfg.SharedDir, ".agentvm/shared") {
		t.Errorf("expected shared dir containing .agentvm/shared, got %s", cfg.SharedDir)
	}
}
