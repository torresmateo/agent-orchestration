package ws

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Client is a WebSocket client managed by the Hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte

	subMu         sync.RWMutex
	subscriptions map[string]bool
}

// NewClient creates a new WebSocket client.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}
}

// Subscribe adds a channel subscription.
func (c *Client) Subscribe(channel string) {
	c.subMu.Lock()
	c.subscriptions[channel] = true
	c.subMu.Unlock()

	// If subscribing to a log channel, start the stream
	if agentID := parseLogChannel(channel); agentID != "" {
		c.hub.LogManager().Subscribe(agentID, c)
	}
}

// Unsubscribe removes a channel subscription.
func (c *Client) Unsubscribe(channel string) {
	c.subMu.Lock()
	delete(c.subscriptions, channel)
	c.subMu.Unlock()
}

// IsSubscribed checks if the client is subscribed to a channel.
func (c *Client) IsSubscribed(channel string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.subscriptions[channel]
}

// Subscriptions returns a copy of the current subscriptions.
func (c *Client) Subscriptions() []string {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	result := make([]string, 0, len(c.subscriptions))
	for ch := range c.subscriptions {
		result = append(result, ch)
	}
	return result
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		var env Envelope
		if err := json.Unmarshal(message, &env); err != nil {
			log.Printf("WebSocket: invalid message: %v", err)
			continue
		}

		c.hub.HandleClientMessage(c, env)
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send queues a message for sending to the client.
func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
	default:
		// Client send buffer full
	}
}

// parseLogChannel extracts agentID from "logs:<agentID>" channel names.
func parseLogChannel(channel string) string {
	if strings.HasPrefix(channel, "logs:") {
		return channel[5:]
	}
	return ""
}

// unmarshalPayload is a helper to unmarshal a JSON payload.
func unmarshalPayload(data json.RawMessage, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("WebSocket: failed to unmarshal payload: %v", err)
		return err
	}
	return nil
}
