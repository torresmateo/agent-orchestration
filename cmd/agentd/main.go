package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mateo/agentvm/internal/api"
	"github.com/mateo/agentvm/internal/config"
	"github.com/mateo/agentvm/internal/lima"
	"github.com/mateo/agentvm/internal/network"
	"github.com/mateo/agentvm/internal/orchestrator"
	"github.com/mateo/agentvm/internal/pool"
	"github.com/mateo/agentvm/internal/registry"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("agentd starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := config.EnsureDirs(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	limaClient, err := lima.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Lima client: %v", err)
	}

	// Pool manager
	poolMgr, err := pool.NewManager(pool.PoolConfig{
		WarmSize:   cfg.Pool.WarmSize,
		MaxVMs:     cfg.Pool.MaxVMs,
		MasterName: cfg.VM.Master,
	}, limaClient, config.BaseDir())
	if err != nil {
		log.Fatalf("Failed to create pool manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poolMgr.Start(ctx)

	// Registry store
	store, err := registry.NewStore(config.BaseDir())
	if err != nil {
		log.Fatalf("Failed to create registry store: %v", err)
	}

	// Traefik writer
	traefikWriter := network.NewTraefikWriter(config.BaseDir(), cfg.Network.Domain)

	// Registration server (port 8090 — VMs call this)
	regServer := registry.NewServer(store, func(reg *registry.AgentRegistration) {
		if err := traefikWriter.WriteRoute(reg); err != nil {
			log.Printf("Failed to write Traefik route for %s: %v", reg.AgentID, err)
		} else {
			log.Printf("Traefik route written for %s -> %s", reg.AgentID, reg.VMIP)
		}
	})

	// Orchestrator
	hostAddr := fmt.Sprintf("host.lima.internal:%d", cfg.Network.RegistryPort)
	orch := orchestrator.New(poolMgr, limaClient, config.BaseDir(), hostAddr)

	// Monitor
	monitor := orchestrator.NewMonitor(poolMgr, limaClient, 15*time.Second)
	monitor.Start(ctx)

	// API server (port 8091 — agentctl calls this)
	apiMux := http.NewServeMux()
	setupAPIRoutes(apiMux, orch, poolMgr, store, traefikWriter, cfg)

	// Start servers
	go func() {
		addr := registry.FormatAddr(cfg.Network.RegistryPort)
		if err := registry.ListenAndServe(addr, regServer); err != nil {
			log.Fatalf("Registration server failed: %v", err)
		}
	}()

	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", cfg.API.Port)
		log.Printf("API server listening on %s", addr)
		srv := &http.Server{
			Addr:         addr,
			Handler:      apiMux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	// Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	log.Println("Shutting down...")
	monitor.Stop()
	poolMgr.Stop()
	cancel()
}

func setupAPIRoutes(mux *http.ServeMux, orch *orchestrator.Orchestrator, poolMgr *pool.Manager, store *registry.Store, tw *network.TraefikWriter, cfg config.Config) {
	// POST /dispatch
	mux.HandleFunc("POST /dispatch", func(w http.ResponseWriter, r *http.Request) {
		var req api.DispatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}

		result, err := orch.Dispatch(r.Context(), orchestrator.DispatchRequest{
			Project:   req.Project,
			RepoURL:   req.RepoURL,
			Issue:     req.Issue,
			Tool:      req.Tool,
			Prompt:    req.Prompt,
			Branch:    req.Branch,
			MaxTime:   req.MaxTime,
			MaxTokens: req.MaxTokens,
			EnvVars:   req.EnvVars,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, api.DispatchResponse{
			AgentID:   result.AgentID,
			VMName:    result.VMName,
			VMIP:      result.VMIP,
			Subdomain: tw.SubdomainFor(result.AgentID, req.Project),
		})
	})

	// GET /status
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		warm, active, cold := poolMgr.Status()
		agents := poolMgr.ActiveSlots()

		statusAgents := make([]api.AgentStatus, 0, len(agents))
		for _, slot := range agents {
			statusAgents = append(statusAgents, api.AgentStatus{
				AgentID:   slot.AgentID,
				VMName:    slot.Name,
				VMIP:      slot.VMIP,
				Project:   slot.Project,
				State:     string(slot.State),
				StartedAt: slot.ClaimedAt,
				Elapsed:   time.Since(slot.ClaimedAt),
				Subdomain: tw.SubdomainFor(slot.AgentID, slot.Project),
			})
		}

		writeJSON(w, http.StatusOK, api.PoolStatus{
			Warm:   warm,
			Active: active,
			Cold:   cold,
			Agents: statusAgents,
		})
	})

	// POST /agents/{id}/kill
	mux.HandleFunc("POST /agents/{id}/kill", func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		slot, ok := poolMgr.GetSlot(agentID)
		if !ok {
			writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "agent not found"})
			return
		}

		// Stop the harness, release the VM
		if err := poolMgr.Release(slot.Name); err != nil {
			writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}
		tw.RemoveRoute(agentID)
		store.Deregister(agentID)
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	// GET /agents/{id}/logs
	mux.HandleFunc("GET /agents/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		slot, ok := poolMgr.GetSlot(agentID)
		if !ok {
			writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "agent not found"})
			return
		}

		limaClient, err := lima.NewClient()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}

		output, err := limaClient.Shell(r.Context(), lima.ShellOptions{
			Instance: slot.Name,
			Command:  "sudo",
			Args:     []string{"journalctl", "-u", "agent-harness.service", "--no-pager", "-n", "200"},
			Timeout:  15 * time.Second,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error()})
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(output))
	})

	// POST /pool/replenish
	mux.HandleFunc("POST /pool/replenish", func(w http.ResponseWriter, r *http.Request) {
		go poolMgr.Replenish(context.Background())
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	// POST /pool/drain
	mux.HandleFunc("POST /pool/drain", func(w http.ResponseWriter, r *http.Request) {
		go poolMgr.Drain(context.Background())
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	// POST /pool/resize
	mux.HandleFunc("POST /pool/resize", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WarmSize int `json:"warmSize"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
			return
		}
		poolMgr.Resize(req.WarmSize)
		go poolMgr.Replenish(context.Background())
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	// GET /health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
