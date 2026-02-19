package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mateo/agentvm/internal/pool"
	"github.com/mateo/agentvm/internal/registry"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Hub manages all WebSocket clients and broadcasts.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool

	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte

	store      *registry.Store
	poolMgr    *pool.Manager
	logMgr     *LogStreamManager
	cmdHandler *CommandHandler

	stopCh chan struct{}
}

// NewHub creates a new WebSocket hub.
func NewHub(store *registry.Store, poolMgr *pool.Manager, cmdHandler *CommandHandler) *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		store:      store,
		poolMgr:    poolMgr,
		cmdHandler: cmdHandler,
		stopCh:     make(chan struct{}),
	}
	h.logMgr = NewLogStreamManager(h)
	return h
}

// Run starts the hub's main event loop. Call in a goroutine.
func (h *Hub) Run() {
	// Subscribe to registry events
	events := h.store.Subscribe()
	defer h.store.Unsubscribe(events)

	// Periodic status snapshot (every 5s)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Clean up any log subscriptions
				h.logMgr.UnsubscribeAll(client)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Client too slow, will be cleaned up
				}
			}
			h.mu.RUnlock()

		case event := <-events:
			h.handleStoreEvent(event)

		case <-ticker.C:
			h.broadcastStatusSnapshot()
		}
	}
}

// Stop shuts down the hub.
func (h *Hub) Stop() {
	close(h.stopCh)
	h.logMgr.StopAll()
}

// ServeWS handles the WebSocket upgrade and creates a client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := NewClient(h, conn)
	h.register <- client

	go client.writePump()
	go client.readPump()
}

// HandleClientMessage processes a parsed message from a client.
func (h *Hub) HandleClientMessage(client *Client, env Envelope) {
	switch env.Type {
	case TypeSubscribe:
		var payload SubscribePayload
		if err := unmarshalPayload(env.Payload, &payload); err != nil {
			return
		}
		client.Subscribe(payload.Channel)

		// If subscribing to status, send initial snapshot
		if payload.Channel == ChannelStatus {
			if msg := h.buildStatusSnapshot(); msg != nil {
				client.send <- msg
			}
		}

	case TypeUnsubscribe:
		var payload UnsubscribePayload
		if err := unmarshalPayload(env.Payload, &payload); err != nil {
			return
		}
		client.Unsubscribe(payload.Channel)

		// If it's a log channel, clean up the stream
		if agentID := parseLogChannel(payload.Channel); agentID != "" {
			h.logMgr.Unsubscribe(agentID, client)
		}

	case TypeCommand:
		var payload CommandPayload
		if err := unmarshalPayload(env.Payload, &payload); err != nil {
			return
		}
		go h.cmdHandler.Handle(client, payload)
	}
}

// SendToLogSubscribers sends a log line to all clients subscribed to that agent's logs.
func (h *Hub) SendToLogSubscribers(agentID, line string) {
	msg, err := MakeEnvelope(TypeLogsData, LogDataPayload{
		AgentID: agentID,
		Line:    line,
	})
	if err != nil {
		return
	}

	channel := "logs:" + agentID

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.IsSubscribed(channel) {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}

// LogManager returns the hub's log stream manager.
func (h *Hub) LogManager() *LogStreamManager {
	return h.logMgr
}

func (h *Hub) handleStoreEvent(event registry.StoreEvent) {
	switch event.Type {
	case registry.EventAgentRegistered:
		snap := agentRegToSnapshot(event.Agent)
		msg, err := MakeEnvelope(TypeAgentRegistered, AgentEventPayload{
			AgentID: event.AgentID,
			Agent:   snap,
		})
		if err != nil {
			return
		}
		h.broadcastToStatusSubscribers(msg)

	case registry.EventAgentDeregistered:
		msg, err := MakeEnvelope(TypeAgentDeregistered, AgentEventPayload{
			AgentID: event.AgentID,
		})
		if err != nil {
			return
		}
		h.broadcastToStatusSubscribers(msg)

	case registry.EventAgentUpdated:
		if event.Agent == nil {
			return
		}
		msg, err := MakeEnvelope(TypeStatusUpdate, StatusUpdatePayload{
			AgentID: event.AgentID,
			State:   event.Agent.State,
			Message: event.Agent.Message,
			Branch:  event.Agent.Branch,
		})
		if err != nil {
			return
		}
		h.broadcastToStatusSubscribers(msg)
	}
}

func (h *Hub) broadcastToStatusSubscribers(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.IsSubscribed(ChannelStatus) {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}

func (h *Hub) broadcastStatusSnapshot() {
	msg := h.buildStatusSnapshot()
	if msg == nil {
		return
	}
	h.broadcastToStatusSubscribers(msg)
}

func (h *Hub) buildStatusSnapshot() []byte {
	warm, active, cold := h.poolMgr.Status()
	activeSlots := h.poolMgr.ActiveSlots()
	registrations := h.store.List()

	// Build a map of registry data for enrichment
	regMap := make(map[string]*registry.AgentRegistration)
	for _, r := range registrations {
		regMap[r.AgentID] = r
	}

	agents := make([]AgentSnapshot, 0, len(activeSlots))
	for _, slot := range activeSlots {
		snap := AgentSnapshot{
			AgentID:   slot.AgentID,
			VMName:    slot.Name,
			VMIP:      slot.VMIP,
			Project:   slot.Project,
			Tool:      slot.Tool,
			Branch:    slot.Branch,
			Issue:     slot.Issue,
			State:     string(slot.State),
			StartedAt: slot.ClaimedAt,
			Elapsed:   time.Since(slot.ClaimedAt).Truncate(time.Second).String(),
		}
		// Enrich with registry data
		if reg, ok := regMap[slot.AgentID]; ok {
			snap.State = reg.State
			snap.Message = reg.Message
			if reg.Branch != "" {
				snap.Branch = reg.Branch
			}
		}
		agents = append(agents, snap)
	}

	msg, err := MakeEnvelope(TypeStatusSnapshot, StatusSnapshotPayload{
		Pool: PoolSnapshot{
			Warm:   warm,
			Active: active,
			Cold:   cold,
		},
		Agents: agents,
	})
	if err != nil {
		return nil
	}
	return msg
}

func agentRegToSnapshot(reg *registry.AgentRegistration) *AgentSnapshot {
	if reg == nil {
		return nil
	}
	return &AgentSnapshot{
		AgentID:   reg.AgentID,
		VMName:    reg.VMName,
		VMIP:      reg.VMIP,
		Project:   reg.Project,
		Tool:      reg.Tool,
		Branch:    reg.Branch,
		State:     reg.State,
		Message:   reg.Message,
		StartedAt: reg.RegisteredAt,
		Elapsed:   time.Since(reg.RegisteredAt).Truncate(time.Second).String(),
	}
}
