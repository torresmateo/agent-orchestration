package orchestrator

import (
	"context"
	"log"
	"time"

	"github.com/mateo/agentvm/internal/lima"
	"github.com/mateo/agentvm/internal/pool"
)

type Monitor struct {
	pool       *pool.Manager
	limaClient lima.Client
	interval   time.Duration
	stopCh     chan struct{}
}

func NewMonitor(pm *pool.Manager, lc lima.Client, interval time.Duration) *Monitor {
	return &Monitor{
		pool:       pm,
		limaClient: lc,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	go m.loop(ctx)
}

func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) loop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAgents(ctx)
		}
	}
}

func (m *Monitor) checkAgents(ctx context.Context) {
	active := m.pool.ActiveSlots()
	for _, slot := range active {
		// Check if the VM is still running
		inst, err := m.limaClient.Get(ctx, slot.Name)
		if err != nil {
			log.Printf("Monitor: VM %s not found, releasing slot", slot.Name)
			m.pool.Release(slot.Name)
			continue
		}

		if inst.Status != lima.StatusRunning {
			log.Printf("Monitor: VM %s is %s, releasing slot", slot.Name, inst.Status)
			m.pool.Release(slot.Name)
			continue
		}

		// Check if the harness service is still running
		output, err := m.limaClient.Shell(ctx, lima.ShellOptions{
			Instance: slot.Name,
			Command:  "systemctl",
			Args:     []string{"is-active", "agent-harness.service"},
			Timeout:  10 * time.Second,
		})
		if err != nil || (output != "active\n" && output != "activating\n") {
			log.Printf("Monitor: harness on %s is not active (output: %q), agent may have completed", slot.Name, output)
		}
	}
}
