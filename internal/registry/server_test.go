package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer(t *testing.T) (*Server, *Store) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	var lastRegistered *AgentRegistration
	server := NewServer(store, func(reg *AgentRegistration) {
		lastRegistered = reg
	})
	_ = lastRegistered

	return server, store
}

func TestServer_Register(t *testing.T) {
	srv, store := setupTestServer(t)

	req := RegisterRequest{
		AgentID: "agent-1",
		VMName:  "warm-1",
		VMIP:    "192.168.64.5",
		Project: "testproject",
		Tool:    "claude-code",
		Ports:   []int{8080},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp RegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.OK {
		t.Error("expected OK response")
	}

	// Verify in store
	reg, ok := store.Get("agent-1")
	if !ok {
		t.Fatal("agent not found in store")
	}
	if reg.VMIP != "192.168.64.5" {
		t.Errorf("expected IP 192.168.64.5, got %s", reg.VMIP)
	}
	if reg.State != "registered" {
		t.Errorf("expected state registered, got %s", reg.State)
	}
}

func TestServer_Deregister(t *testing.T) {
	srv, store := setupTestServer(t)

	// First register
	store.Register(&AgentRegistration{AgentID: "agent-1", State: "registered"})

	req := DeregisterRequest{AgentID: "agent-1"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/deregister", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	_, ok := store.Get("agent-1")
	if ok {
		t.Error("agent should have been deregistered")
	}
}

func TestServer_ListAgents(t *testing.T) {
	srv, store := setupTestServer(t)

	store.Register(&AgentRegistration{AgentID: "agent-1", State: "registered"})
	store.Register(&AgentRegistration{AgentID: "agent-2", State: "running"})

	httpReq := httptest.NewRequest("GET", "/agents", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var agents []*AgentRegistration
	json.NewDecoder(w.Body).Decode(&agents)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestServer_Health(t *testing.T) {
	srv, _ := setupTestServer(t)

	httpReq := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestServer_StatusUpdate(t *testing.T) {
	srv, store := setupTestServer(t)

	store.Register(&AgentRegistration{AgentID: "agent-1", State: "registered"})

	payload := map[string]string{
		"agentID": "agent-1",
		"state":   "running",
		"message": "Executing claude-code",
	}
	body, _ := json.Marshal(payload)
	httpReq := httptest.NewRequest("POST", "/status", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	reg, _ := store.Get("agent-1")
	if reg.State != "running" {
		t.Errorf("expected running, got %s", reg.State)
	}
}
