package ws

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"sync"
)

// logStream tracks a single journalctl -f process and its subscribers.
type logStream struct {
	agentID string
	vmName  string
	cancel  context.CancelFunc
	mu      sync.Mutex
	clients map[*Client]bool
}

// LogStreamManager manages per-agent log streaming processes.
type LogStreamManager struct {
	hub     *Hub
	mu      sync.Mutex
	streams map[string]*logStream // keyed by agentID
}

// NewLogStreamManager creates a new manager.
func NewLogStreamManager(hub *Hub) *LogStreamManager {
	return &LogStreamManager{
		hub:     hub,
		streams: make(map[string]*logStream),
	}
}

// Subscribe adds a client to an agent's log stream, starting it if needed.
func (m *LogStreamManager) Subscribe(agentID string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[agentID]
	if !ok {
		// Look up VM name from pool
		slot, found := m.hub.poolMgr.GetSlot(agentID)
		if !found {
			log.Printf("LogStream: agent %s not found in pool", agentID)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		stream = &logStream{
			agentID: agentID,
			vmName:  slot.Name,
			cancel:  cancel,
			clients: make(map[*Client]bool),
		}
		m.streams[agentID] = stream
		go m.runStream(ctx, stream)
	}

	stream.mu.Lock()
	stream.clients[client] = true
	stream.mu.Unlock()
}

// Unsubscribe removes a client from an agent's log stream.
// Stops the stream if no subscribers remain.
func (m *LogStreamManager) Unsubscribe(agentID string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.streams[agentID]
	if !ok {
		return
	}

	stream.mu.Lock()
	delete(stream.clients, client)
	remaining := len(stream.clients)
	stream.mu.Unlock()

	if remaining == 0 {
		stream.cancel()
		delete(m.streams, agentID)
		log.Printf("LogStream: stopped stream for %s (no subscribers)", agentID)
	}
}

// UnsubscribeAll removes a client from all streams it's subscribed to.
func (m *LogStreamManager) UnsubscribeAll(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for agentID, stream := range m.streams {
		stream.mu.Lock()
		delete(stream.clients, client)
		remaining := len(stream.clients)
		stream.mu.Unlock()

		if remaining == 0 {
			stream.cancel()
			delete(m.streams, agentID)
		}
	}
}

// StopAll cancels all running streams.
func (m *LogStreamManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stream := range m.streams {
		stream.cancel()
	}
	m.streams = make(map[string]*logStream)
}

// runStream executes journalctl -f via limactl shell and fans out lines.
func (m *LogStreamManager) runStream(ctx context.Context, stream *logStream) {
	log.Printf("LogStream: starting for agent %s (VM: %s)", stream.agentID, stream.vmName)

	cmd := exec.CommandContext(ctx, "limactl", "shell", stream.vmName,
		"sudo", "journalctl", "-u", "agent-harness.service", "-f", "--no-pager", "-n", "100")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("LogStream: pipe error for %s: %v", stream.agentID, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("LogStream: start error for %s: %v", stream.agentID, err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		m.hub.SendToLogSubscribers(stream.agentID, line)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == nil {
			log.Printf("LogStream: process exited for %s: %v", stream.agentID, err)
		}
	}

	log.Printf("LogStream: ended for agent %s", stream.agentID)
}
