package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"browser-manager/internal/browser"
	"browser-manager/internal/cdp"
)

// Server provides an HTTP REST API for browser lifecycle management.
type Server struct {
	manager *browser.BrowserManager
	addr    string
	http    *http.Server
}

// NewServer creates a new Server that listens on addr and delegates browser
// operations to the supplied BrowserManager.
func NewServer(addr string, manager *browser.BrowserManager) *Server {
	s := &Server{
		manager: manager,
		addr:    addr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/ws-url", s.handleWsURL)
	mux.HandleFunc("POST /api/restart", s.handleRestart)
	mux.HandleFunc("POST /api/stop", s.handleStop)

	s.http = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start begins listening for HTTP requests. This call blocks until the server
// encounters an error or is shut down via Stop.
func (s *Server) Start() error {
	log.Printf("Server listening on %s", s.addr)
	return s.http.ListenAndServe()
}

// Stop performs a graceful shutdown of the HTTP server with a 5-second timeout.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}

// handleStatus returns the current browser status as JSON.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.manager.Status()
	writeJSON(w, http.StatusOK, status)
}

// handleWsURL returns the CDP WebSocket debug URL for the running browser.
// Returns 503 if the browser is not currently running.
func (s *Server) handleWsURL(w http.ResponseWriter, r *http.Request) {
	if !s.manager.IsRunning() {
		writeError(w, http.StatusServiceUnavailable, "browser not running")
		return
	}

	status := s.manager.Status()
	client := cdp.NewCDPClient("127.0.0.1", status.DebugPort)
	wsURL, err := client.GetWebSocketURL()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"ws_url": wsURL})
}

// handleRestart restarts the browser and returns the new status.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if err := s.manager.Restart(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	status := s.manager.Status()
	writeJSON(w, http.StatusOK, status)
}

// handleStop stops the browser and shuts down the server. The HTTP response is
// sent before shutdown begins so the caller receives confirmation.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "server shutting down"})

	go func() {
		if err := s.manager.Stop(); err != nil {
			log.Printf("Browser stop error: %v", err)
		}
		s.Stop()
	}()
}

// writeJSON sets the response Content-Type to application/json, writes the
// status code, and encodes data as JSON.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// writeError is a convenience wrapper around writeJSON that sends an error
// response in the form {"error": message}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
