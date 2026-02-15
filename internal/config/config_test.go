package config

import (
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Pool.WarmSize != 3 {
		t.Errorf("expected warmSize 3, got %d", cfg.Pool.WarmSize)
	}
	if cfg.Pool.MaxVMs != 15 {
		t.Errorf("expected maxVMs 15, got %d", cfg.Pool.MaxVMs)
	}
	if cfg.VM.CPUs != 2 {
		t.Errorf("expected 2 CPUs, got %d", cfg.VM.CPUs)
	}
	if cfg.VM.MemoryGiB != 3 {
		t.Errorf("expected 3 GiB, got %d", cfg.VM.MemoryGiB)
	}
	if cfg.VM.Master != "agent-master" {
		t.Errorf("expected agent-master, got %s", cfg.VM.Master)
	}
	if cfg.Network.Domain != "agents.test" {
		t.Errorf("expected agents.test, got %s", cfg.Network.Domain)
	}
	if cfg.Network.RegistryPort != 8090 {
		t.Errorf("expected port 8090, got %d", cfg.Network.RegistryPort)
	}
	if cfg.API.Port != 8091 {
		t.Errorf("expected port 8091, got %d", cfg.API.Port)
	}
}

func TestBaseDir(t *testing.T) {
	dir := BaseDir()
	if dir == "" {
		t.Error("expected non-empty base dir")
	}
}
