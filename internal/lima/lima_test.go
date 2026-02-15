package lima

import (
	"context"
	"testing"
	"time"
)

func TestMockClient_CreateAndList(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	err := mock.Create(ctx, CreateOptions{
		Name:  "test-vm",
		CPUs:  2,
		Start: true,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	instances, err := mock.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].Status != StatusRunning {
		t.Errorf("expected Running, got %s", instances[0].Status)
	}
}

func TestMockClient_Clone(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.Create(ctx, CreateOptions{Name: "master", CPUs: 2})

	err := mock.Clone(ctx, CloneOptions{
		Source: "master",
		Target: "clone-1",
		Start:  true,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	inst, err := mock.Get(ctx, "clone-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if inst.Status != StatusRunning {
		t.Errorf("expected Running, got %s", inst.Status)
	}
}

func TestMockClient_CloneNonexistent(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	err := mock.Clone(ctx, CloneOptions{
		Source: "nonexistent",
		Target: "clone-1",
	})
	if err == nil {
		t.Fatal("expected error cloning nonexistent source")
	}
}

func TestGetVMIP(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.Create(ctx, CreateOptions{Name: "test-vm", Start: true})
	mock.ShellFn = func(ctx context.Context, opts ShellOptions) (string, error) {
		return "192.168.64.5 fd00::1\n", nil
	}

	ip, err := GetVMIP(ctx, mock, "test-vm")
	if err != nil {
		t.Fatalf("GetVMIP failed: %v", err)
	}
	if ip != "192.168.64.5" {
		t.Errorf("expected 192.168.64.5, got %s", ip)
	}
}

func TestCopyOptions(t *testing.T) {
	opts := CopyOptions{
		Instance:  "test-vm",
		Direction: CopyToVM,
		LocalPath: "/tmp/file",
		VMPath:    "/etc/config",
	}
	if opts.Direction != CopyToVM {
		t.Error("expected CopyToVM")
	}
}

func TestShellOptions(t *testing.T) {
	opts := ShellOptions{
		Instance: "test-vm",
		Command:  "echo",
		Args:     []string{"hello"},
		Timeout:  10 * time.Second,
	}
	if opts.Timeout != 10*time.Second {
		t.Error("expected 10s timeout")
	}
}

func TestMockClient_Delete(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.Create(ctx, CreateOptions{Name: "test-vm"})
	if err := mock.Delete(ctx, "test-vm", false); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := mock.Get(ctx, "test-vm")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMockClient_StartStop(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	mock.Create(ctx, CreateOptions{Name: "test-vm"})

	if err := mock.Start(ctx, "test-vm"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	inst, _ := mock.Get(ctx, "test-vm")
	if inst.Status != StatusRunning {
		t.Errorf("expected Running after start, got %s", inst.Status)
	}

	if err := mock.Stop(ctx, "test-vm"); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	inst, _ = mock.Get(ctx, "test-vm")
	if inst.Status != StatusStopped {
		t.Errorf("expected Stopped after stop, got %s", inst.Status)
	}
}
