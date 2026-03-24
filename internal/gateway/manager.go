package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"llm_manager/internal/provider"
)

type Gateway struct {
	Port     int    `json:"port"`
	Provider string `json:"provider"`
}

type Manager struct {
	providers map[string]provider.Provider
	gateways  map[int]*gatewayServer
	mu        sync.RWMutex
}

func NewManager(providers map[string]provider.Provider) *Manager {
	return &Manager{
		providers: providers,
		gateways:  map[int]*gatewayServer{},
	}
}

func (m *Manager) ProviderNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.providers))
	for k := range m.providers {
		out = append(out, k)
	}
	return out
}

func (m *Manager) ListGateways() []Gateway {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Gateway, 0, len(m.gateways))
	for _, gw := range m.gateways {
		out = append(out, Gateway{Port: gw.port, Provider: gw.ProviderName()})
	}
	return out
}

func (m *Manager) CreateGateway(port int, providerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.providers[providerName]; !ok {
		return fmt.Errorf("provider not found: %s", providerName)
	}
	if _, exists := m.gateways[port]; exists {
		return fmt.Errorf("gateway already exists on port %d", port)
	}

	gw := newGatewayServer(port, providerName, m.providers)
	m.gateways[port] = gw

	go func() {
		if err := gw.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("gateway on :%d crashed: %v", port, err)
		}
	}()

	return nil
}

func (m *Manager) SwitchProvider(port int, providerName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.providers[providerName]; !ok {
		return fmt.Errorf("provider not found: %s", providerName)
	}
	gw, ok := m.gateways[port]
	if !ok {
		return fmt.Errorf("gateway not found on port %d", port)
	}
	gw.SetProvider(providerName)
	return nil
}

func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, gw := range m.gateways {
		if err := gw.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

type gatewayServer struct {
	port         int
	providers    map[string]provider.Provider
	providerName string
	mu           sync.RWMutex
	srv          *http.Server
}

type generateRequest struct {
	Prompt string `json:"prompt"`
}

type generateResponse struct {
	Provider string `json:"provider"`
	Output   string `json:"output"`
}

func newGatewayServer(port int, providerName string, providers map[string]provider.Provider) *gatewayServer {
	g := &gatewayServer{
		port:         port,
		providerName: providerName,
		providers:    providers,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", g.handleHealth)
	mux.HandleFunc("POST /v1/generate", g.handleGenerate)
	g.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return g
}

func (g *gatewayServer) Start() error {
	log.Printf("gateway listening on :%d provider=%s", g.port, g.ProviderName())
	return g.srv.ListenAndServe()
}

func (g *gatewayServer) Shutdown(ctx context.Context) error {
	return g.srv.Shutdown(ctx)
}

func (g *gatewayServer) SetProvider(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.providerName = name
}

func (g *gatewayServer) ProviderName() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.providerName
}

func (g *gatewayServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"port":     g.port,
		"provider": g.ProviderName(),
	})
}

func (g *gatewayServer) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	providerName := g.ProviderName()
	p := g.providers[providerName]
	out, err := p.Generate(r.Context(), req.Prompt)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error(), "provider": providerName})
		return
	}
	writeJSON(w, http.StatusOK, generateResponse{Provider: providerName, Output: out})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
