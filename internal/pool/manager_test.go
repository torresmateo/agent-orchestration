package pool

import (
	"context"
	"testing"
	"os"
	"path/filepath"

	"github.com/mateo/agentvm/internal/lima"
)

func setupTestPool(t *testing.T) (*Manager, *lima.MockClient, string) {
	t.Helper()
	tmpDir := t.TempDir()

	mock := lima.NewMockClient()
	ctx := context.Background()

	// Create a master VM
	mock.Create(ctx, lima.CreateOptions{Name: "agent-master", CPUs: 2})

	cfg := PoolConfig{
		WarmSize:   3,
		MaxVMs:     10,
		MasterName: "agent-master",
	}

	mgr, err := NewManager(cfg, mock, tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	return mgr, mock, tmpDir
}

func TestManager_ClaimFromEmpty(t *testing.T) {
	mgr, _, _ := setupTestPool(t)
	ctx := context.Background()

	_, err := mgr.Claim(ctx, "agent-1", "testproject")
	if err == nil {
		t.Fatal("expected error claiming from empty pool")
	}
}

func TestManager_ClaimAndRelease(t *testing.T) {
	mgr, mock, _ := setupTestPool(t)
	ctx := context.Background()

	mock.ShellFn = func(ctx context.Context, opts lima.ShellOptions) (string, error) {
		return "192.168.64.5\n", nil
	}

	// Manually add an idle slot (simulating replenish)
	mgr.mu.Lock()
	mgr.slots = append(mgr.slots, VMSlot{
		Name:  "warm-1",
		State: SlotIdle,
	})
	mgr.mu.Unlock()

	// Add the VM to mock
	mock.Create(ctx, lima.CreateOptions{Name: "warm-1", Start: true})

	// Claim
	slot, err := mgr.Claim(ctx, "agent-1", "testproject")
	if err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	if slot.AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", slot.AgentID)
	}
	if slot.State != SlotActive {
		t.Errorf("expected active, got %s", slot.State)
	}

	// Check status
	warm, active, cold := mgr.Status()
	if warm != 0 || active != 1 || cold != 0 {
		t.Errorf("expected 0/1/0, got %d/%d/%d", warm, active, cold)
	}

	// Release
	err = mgr.Release("warm-1")
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	warm, active, cold = mgr.Status()
	if warm != 0 || active != 0 || cold != 1 {
		t.Errorf("expected 0/0/1, got %d/%d/%d", warm, active, cold)
	}
}

func TestManager_Destroy(t *testing.T) {
	mgr, mock, _ := setupTestPool(t)
	ctx := context.Background()

	mock.Create(ctx, lima.CreateOptions{Name: "warm-1", Start: true})
	mgr.mu.Lock()
	mgr.slots = append(mgr.slots, VMSlot{Name: "warm-1", State: SlotIdle})
	mgr.mu.Unlock()

	err := mgr.Destroy(ctx, "warm-1")
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	warm, active, cold := mgr.Status()
	if warm != 0 || active != 0 || cold != 0 {
		t.Errorf("expected 0/0/0, got %d/%d/%d", warm, active, cold)
	}

	// VM should be deleted from mock
	_, err = mock.Get(ctx, "warm-1")
	if err == nil {
		t.Error("expected VM to be deleted")
	}
}

func TestManager_GetSlot(t *testing.T) {
	mgr, _, _ := setupTestPool(t)

	mgr.mu.Lock()
	mgr.slots = append(mgr.slots, VMSlot{
		Name:    "warm-1",
		State:   SlotActive,
		AgentID: "agent-42",
	})
	mgr.mu.Unlock()

	// Find by VM name
	slot, ok := mgr.GetSlot("warm-1")
	if !ok {
		t.Fatal("expected to find slot by name")
	}
	if slot.AgentID != "agent-42" {
		t.Errorf("expected agent-42, got %s", slot.AgentID)
	}

	// Find by agent ID
	slot, ok = mgr.GetSlot("agent-42")
	if !ok {
		t.Fatal("expected to find slot by agent ID")
	}

	// Not found
	_, ok = mgr.GetSlot("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	state := PersistentState{
		Counter: 5,
		Slots: []VMSlot{
			{Name: "warm-1", State: SlotIdle},
			{Name: "warm-2", State: SlotActive, AgentID: "agent-1"},
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tmpDir, "pool-state.json")); err != nil {
		t.Fatalf("State file not found: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Counter != 5 {
		t.Errorf("expected counter 5, got %d", loaded.Counter)
	}
	if len(loaded.Slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(loaded.Slots))
	}
	if loaded.Slots[1].AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", loaded.Slots[1].AgentID)
	}
}

func TestStateLoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load from empty dir failed: %v", err)
	}
	if len(state.Slots) != 0 {
		t.Errorf("expected empty slots, got %d", len(state.Slots))
	}
}
