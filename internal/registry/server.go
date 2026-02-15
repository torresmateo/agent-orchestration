package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	store    *Store
	onRegister func(reg *AgentRegistration) // callback for traefik config
	mux      *http.ServeMux
}

func NewServer(store *Store, onRegister func(reg *AgentRegistration)) *Server {
	s := &Server{
		store:      store,
		onRegister: onRegister,
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("POST /register", s.handleRegister)
	s.mux.HandleFunc("POST /deregister", s.handleDeregister)
	s.mux.HandleFunc("POST /status", s.handleStatus)
	s.mux.HandleFunc("GET /agents", s.handleListAgents)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Message: err.Error()})
		return
	}

	reg := &AgentRegistration{
		AgentID:       req.AgentID,
		VMName:        req.VMName,
		VMIP:          req.VMIP,
		Project:       req.Project,
		Tool:          req.Tool,
		Ports:         req.Ports,
		State:         "registered",
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	s.store.Register(reg)
	log.Printf("Agent %s registered from %s (project: %s, tool: %s)", req.AgentID, req.VMIP, req.Project, req.Tool)

	if s.onRegister != nil {
		s.onRegister(reg)
	}

	writeJSON(w, http.StatusOK, RegisterResponse{OK: true})
}

func (s *Server) handleDeregister(w http.ResponseWriter, r *http.Request) {
	var req DeregisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, RegisterResponse{OK: false, Message: err.Error()})
		return
	}

	s.store.Deregister(req.AgentID)
	log.Printf("Agent %s deregistered", req.AgentID)
	writeJSON(w, http.StatusOK, RegisterResponse{OK: true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var report struct {
		AgentID string `json:"agentID"`
		State   string `json:"state"`
		Message string `json:"message,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := s.store.UpdateState(report.AgentID, report.State); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("Agent %s status: %s %s", report.AgentID, report.State, report.Message)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents := s.store.List()
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func ListenAndServe(addr string, s *Server) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("Registration server listening on %s", addr)
	return srv.ListenAndServe()
}

func FormatAddr(port int) string {
	return fmt.Sprintf(":%d", port)
}
