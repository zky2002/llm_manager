package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"llm_manager/internal/config"
	"llm_manager/internal/gateway"
	"llm_manager/internal/provider"
)

type createGatewayRequest struct {
	Port     int    `json:"port"`
	Provider string `json:"provider"`
}

type switchProviderRequest struct {
	Provider string `json:"provider"`
}

func main() {
	cfg := config.LoadFromEnv()

	providers := map[string]provider.Provider{}
	if cfg.LlamaCppURL != "" {
		providers["local"] = provider.NewLlamaCPPProvider(cfg.LlamaCppURL)
	}
	if cfg.OnlineBaseURL != "" && cfg.OnlineAPIKey != "" {
		providers["online"] = provider.NewOpenAICompatibleProvider(cfg.OnlineBaseURL, cfg.OnlineAPIKey, cfg.OnlineModel)
	}
	if len(providers) == 0 {
		log.Fatal("no providers configured. set LLAMA_CPP_URL and/or ONLINE_BASE_URL + ONLINE_API_KEY")
	}

	manager := gateway.NewManager(providers)

	for _, gw := range cfg.DefaultGateways {
		if err := manager.CreateGateway(gw.Port, gw.Provider); err != nil {
			log.Fatalf("create default gateway failed: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /gateways", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, manager.ListGateways())
	})

	mux.HandleFunc("POST /gateways", func(w http.ResponseWriter, r *http.Request) {
		var req createGatewayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port must be > 0")
			return
		}
		if err := manager.CreateGateway(req.Port, req.Provider); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"message": "gateway created", "port": req.Port, "provider": req.Provider})
	})

	mux.HandleFunc("PUT /gateways/{port}/provider", func(w http.ResponseWriter, r *http.Request) {
		var req switchProviderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		var port int
		if _, err := fmt.Sscanf(r.PathValue("port"), "%d", &port); err != nil {
			writeError(w, http.StatusBadRequest, "invalid port path param")
			return
		}
		if err := manager.SwitchProvider(port, req.Provider); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "provider switched", "port": port, "provider": req.Provider})
	})

	mux.HandleFunc("GET /providers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"providers": manager.ProviderNames()})
	})

	adminSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("admin API listening on :%d", cfg.AdminPort)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("admin API error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = adminSrv.Shutdown(shutdownCtx)
	_ = manager.ShutdownAll(shutdownCtx)
	wg.Wait()
	log.Printf("gracefully stopped")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write JSON failed: %v", err)
	}
}
