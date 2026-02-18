package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateo/agentvm/internal/registry"
)

func TestTraefikWriter_WriteRoute(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "agentvm")
	os.MkdirAll(filepath.Join(baseDir, "traefik", "dynamic"), 0755)

	tw := NewTraefikWriter(baseDir, "agents.test")

	reg := &registry.AgentRegistration{
		AgentID: "agent-1",
		Project: "myproject",
		VMIP:    "192.168.64.5",
		Ports:   []int{8080},
	}

	if err := tw.WriteRoute(reg); err != nil {
		t.Fatalf("WriteRoute failed: %v", err)
	}

	// Check file was created
	files, _ := filepath.Glob(filepath.Join(baseDir, "traefik", "dynamic", "*.yaml"))
	if len(files) != 1 {
		t.Fatalf("expected 1 config file, got %d", len(files))
	}

	content, _ := os.ReadFile(files[0])
	s := string(content)

	// Verify content
	if !strings.Contains(s, "agent-1.myproject.agents.test") {
		t.Error("expected host rule with correct subdomain")
	}
	if !strings.Contains(s, "192.168.64.5:8080") {
		t.Error("expected backend URL with VM IP")
	}
	if !strings.Contains(s, "websecure") {
		t.Error("expected websecure entrypoint")
	}
	if !strings.Contains(s, "tls: {}") {
		t.Error("expected tls config in non-httpOnly mode")
	}
}

func TestTraefikWriter_WriteRouteHTTPOnly(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "agentvm")
	os.MkdirAll(filepath.Join(baseDir, "traefik", "dynamic"), 0755)

	tw := NewTraefikWriterHTTPOnly(baseDir, "agents.test", true)

	reg := &registry.AgentRegistration{
		AgentID: "agent-1",
		Project: "myproject",
		VMIP:    "192.168.64.5",
		Ports:   []int{8080},
	}

	if err := tw.WriteRoute(reg); err != nil {
		t.Fatalf("WriteRoute failed: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(baseDir, "traefik", "dynamic", "*.yaml"))
	if len(files) != 1 {
		t.Fatalf("expected 1 config file, got %d", len(files))
	}

	content, _ := os.ReadFile(files[0])
	s := string(content)

	if !strings.Contains(s, "agent-1.myproject.agents.test") {
		t.Error("expected host rule with correct subdomain")
	}
	if !strings.Contains(s, "192.168.64.5:8080") {
		t.Error("expected backend URL with VM IP")
	}
	if strings.Contains(s, "websecure") {
		t.Error("should NOT have websecure entrypoint in HTTP-only mode")
	}
	if !strings.Contains(s, "web") {
		t.Error("expected web entrypoint in HTTP-only mode")
	}
	if strings.Contains(s, "tls:") {
		t.Error("should NOT have tls config in HTTP-only mode")
	}
}

func TestTraefikWriter_RemoveRoute(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "agentvm")
	os.MkdirAll(filepath.Join(baseDir, "traefik", "dynamic"), 0755)

	tw := NewTraefikWriter(baseDir, "agents.test")

	reg := &registry.AgentRegistration{
		AgentID: "agent-1",
		Project: "myproject",
		VMIP:    "192.168.64.5",
	}
	tw.WriteRoute(reg)

	// Remove
	if err := tw.RemoveRoute("agent-1"); err != nil {
		t.Fatalf("RemoveRoute failed: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(baseDir, "traefik", "dynamic", "*.yaml"))
	if len(files) != 0 {
		t.Fatalf("expected 0 config files after removal, got %d", len(files))
	}
}

func TestTraefikWriter_RemoveNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	tw := NewTraefikWriter(tmpDir, "agents.test")

	// Should not error
	if err := tw.RemoveRoute("nonexistent"); err != nil {
		t.Fatalf("RemoveRoute should not error for nonexistent: %v", err)
	}
}

func TestTraefikWriter_SubdomainFor(t *testing.T) {
	tw := NewTraefikWriter("/tmp", "agents.test")
	sub := tw.SubdomainFor("agent-1", "myproject")
	if sub != "agent-1.myproject.agents.test" {
		t.Errorf("expected agent-1.myproject.agents.test, got %s", sub)
	}
}

func TestWriteStaticConfig(t *testing.T) {
	tmpDir := t.TempDir()

	if err := WriteStaticConfig(tmpDir, 80, 443); err != nil {
		t.Fatalf("WriteStaticConfig failed: %v", err)
	}

	path := filepath.Join(tmpDir, "traefik", "traefik.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading static config: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, ":80") {
		t.Error("expected HTTP port 80")
	}
	if !strings.Contains(s, ":443") {
		t.Error("expected HTTPS port 443")
	}
	if !strings.Contains(s, "watch: true") {
		t.Error("expected file provider with watch")
	}
	if !strings.Contains(s, "tls:") {
		t.Error("expected TLS config in default mode")
	}
}

func TestWriteStaticConfigHTTPOnly(t *testing.T) {
	tmpDir := t.TempDir()

	if err := WriteStaticConfigHTTPOnly(tmpDir, 80); err != nil {
		t.Fatalf("WriteStaticConfigHTTPOnly failed: %v", err)
	}

	path := filepath.Join(tmpDir, "traefik", "traefik.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading static config: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, ":80") {
		t.Error("expected HTTP port 80")
	}
	if strings.Contains(s, ":443") {
		t.Error("should NOT have HTTPS port in HTTP-only mode")
	}
	if !strings.Contains(s, "watch: true") {
		t.Error("expected file provider with watch")
	}
	if strings.Contains(s, "tls:") {
		t.Error("should NOT have TLS config in HTTP-only mode")
	}
	if strings.Contains(s, "websecure") {
		t.Error("should NOT have websecure in HTTP-only mode")
	}
	if !strings.Contains(s, "HTTP-only") {
		t.Error("expected HTTP-only comment")
	}
}
