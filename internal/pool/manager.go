package pool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mateo/agentvm/internal/lima"
)

type Manager struct {
	cfg     PoolConfig
	client  lima.Client
	store   *stateStore
	mu      sync.Mutex
	slots   []VMSlot
	counter int
	stopCh  chan struct{}
}

func NewManager(cfg PoolConfig, client lima.Client, baseDir string) (*Manager, error) {
	store := newStateStore(baseDir)
	state, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("loading pool state: %w", err)
	}

	m := &Manager{
		cfg:     cfg,
		client:  client,
		store:   store,
		slots:   state.Slots,
		counter: state.Counter,
		stopCh:  make(chan struct{}),
	}

	return m, nil
}

func (m *Manager) Start(ctx context.Context) {
	// Reconcile existing state with actual Lima VMs
	m.reconcile(ctx)

	// Start background replenish loop
	go m.replenishLoop(ctx)
}

func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) Claim(ctx context.Context, agentID, project string) (*VMSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.slots {
		if m.slots[i].State == SlotIdle {
			m.slots[i].State = SlotActive
			m.slots[i].AgentID = agentID
			m.slots[i].Project = project
			m.slots[i].ClaimedAt = time.Now()

			// Get the VM IP
			ip, err := lima.GetVMIP(ctx, m.client, m.slots[i].Name)
			if err != nil {
				log.Printf("Warning: could not get IP for %s: %v", m.slots[i].Name, err)
			} else {
				m.slots[i].VMIP = ip
			}

			slot := m.slots[i]
			if err := m.persist(); err != nil {
				log.Printf("Warning: failed to persist state: %v", err)
			}

			// Trigger async replenish
			go m.Replenish(context.Background())

			return &slot, nil
		}
	}

	return nil, fmt.Errorf("no warm VMs available. Try again later or run 'agentctl pool replenish'")
}

func (m *Manager) Release(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.slots {
		if m.slots[i].Name == name {
			m.slots[i].State = SlotCold
			m.slots[i].AgentID = ""
			m.slots[i].Project = ""
			return m.persist()
		}
	}
	return fmt.Errorf("VM %q not found in pool", name)
}

func (m *Manager) Destroy(ctx context.Context, name string) error {
	m.mu.Lock()

	idx := -1
	for i := range m.slots {
		if m.slots[i].Name == name {
			idx = i
			break
		}
	}

	if idx < 0 {
		m.mu.Unlock()
		return fmt.Errorf("VM %q not found in pool", name)
	}

	// Remove from slots
	m.slots = append(m.slots[:idx], m.slots[idx+1:]...)
	if err := m.persist(); err != nil {
		log.Printf("Warning: failed to persist state: %v", err)
	}
	m.mu.Unlock()

	// Delete the actual VM
	if err := m.client.Delete(ctx, name, true); err != nil {
		return fmt.Errorf("deleting VM %s: %w", name, err)
	}
	return nil
}

func (m *Manager) Replenish(ctx context.Context) {
	m.mu.Lock()
	warmCount := 0
	total := len(m.slots)
	for _, s := range m.slots {
		if s.State == SlotIdle {
			warmCount++
		}
	}
	needed := m.cfg.WarmSize - warmCount
	if needed <= 0 || total+needed > m.cfg.MaxVMs {
		if total+needed > m.cfg.MaxVMs {
			needed = m.cfg.MaxVMs - total
		}
		if needed <= 0 {
			m.mu.Unlock()
			return
		}
	}
	m.mu.Unlock()

	log.Printf("Pool replenish: creating %d warm VMs", needed)

	for i := 0; i < needed; i++ {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
		}

		m.mu.Lock()
		m.counter++
		name := fmt.Sprintf("warm-%d", m.counter)
		m.slots = append(m.slots, VMSlot{
			Name:      name,
			State:     SlotCreating,
			CreatedAt: time.Now(),
		})
		m.persist()
		m.mu.Unlock()

		log.Printf("Cloning %s from %s...", name, m.cfg.MasterName)
		err := m.client.Clone(ctx, lima.CloneOptions{
			Source:  m.cfg.MasterName,
			Target:  name,
			Start:   true,
			Timeout: 5 * time.Minute,
		})

		m.mu.Lock()
		if err != nil {
			log.Printf("Failed to create %s: %v", name, err)
			// Remove the failed slot
			for j := range m.slots {
				if m.slots[j].Name == name {
					m.slots = append(m.slots[:j], m.slots[j+1:]...)
					break
				}
			}
		} else {
			for j := range m.slots {
				if m.slots[j].Name == name {
					m.slots[j].State = SlotIdle
					break
				}
			}
			log.Printf("Warm VM %s ready", name)
		}
		m.persist()
		m.mu.Unlock()
	}
}

func (m *Manager) Drain(ctx context.Context) error {
	m.mu.Lock()
	var toDestroy []string
	for _, s := range m.slots {
		if s.State == SlotIdle {
			toDestroy = append(toDestroy, s.Name)
		}
	}
	m.mu.Unlock()

	for _, name := range toDestroy {
		if err := m.Destroy(ctx, name); err != nil {
			log.Printf("Warning: failed to destroy %s: %v", name, err)
		}
	}
	return nil
}

func (m *Manager) Resize(warmSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.WarmSize = warmSize
}

func (m *Manager) Status() (warm, active, cold int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.slots {
		switch s.State {
		case SlotIdle:
			warm++
		case SlotActive:
			active++
		case SlotCold:
			cold++
		}
	}
	return
}

func (m *Manager) ActiveSlots() []VMSlot {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []VMSlot
	for _, s := range m.slots {
		if s.State == SlotActive {
			result = append(result, s)
		}
	}
	return result
}

func (m *Manager) GetSlot(name string) (*VMSlot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.slots {
		if s.Name == name || s.AgentID == name {
			return &s, true
		}
	}
	return nil, false
}

func (m *Manager) reconcile(ctx context.Context) {
	instances, err := m.client.List(ctx)
	if err != nil {
		log.Printf("Warning: could not list VMs for reconciliation: %v", err)
		return
	}

	limaVMs := make(map[string]lima.InstanceStatus)
	for _, inst := range instances {
		limaVMs[inst.Name] = inst.Status
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove slots for VMs that no longer exist
	var kept []VMSlot
	for _, s := range m.slots {
		if _, exists := limaVMs[s.Name]; exists {
			kept = append(kept, s)
		} else {
			log.Printf("Reconcile: removing stale slot %s", s.Name)
		}
	}
	m.slots = kept
	m.persist()
}

func (m *Manager) replenishLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial replenish
	m.Replenish(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.Replenish(ctx)
		}
	}
}

func (m *Manager) persist() error {
	return m.store.Save(PersistentState{
		Slots:   m.slots,
		Counter: m.counter,
	})
}
