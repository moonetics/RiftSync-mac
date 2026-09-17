package profilediscovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	Address = "127.0.0.1:8749"
	Path    = "/riftsync/profiles"
)

type Profile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Token   string `json:"token,omitempty"`
	Running bool   `json:"running"`
}

type Snapshot struct {
	Profiles          []Profile `json:"profiles"`
	SelectedProfileID string    `json:"selected_profile_id"`
}

type Provider func() Snapshot

type Service struct {
	server   *http.Server
	listener net.Listener
}

func Start(ctx context.Context, provider Provider) (*Service, error) {
	listener, err := net.Listen("tcp", Address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", Address, err)
	}
	service := &Service{
		server:   &http.Server{Handler: NewHandler(provider)},
		listener: listener,
	}
	go func() {
		_ = service.server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = service.Close(shutdownCtx)
	}()
	return service, nil
}

func (s *Service) Close(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	err := s.server.Shutdown(ctx)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func NewHandler(provider Provider) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(Path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
			return
		}
		if !isLoopbackRequest(r) {
			writeJSON(w, http.StatusForbidden, map[string]any{"status": "error", "error": "local requests only"})
			return
		}
		snapshot := Snapshot{Profiles: []Profile{}}
		if provider != nil {
			snapshot = provider()
		}
		if snapshot.Profiles == nil {
			snapshot.Profiles = []Profile{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":              "ok",
			"profiles":            snapshot.Profiles,
			"selected_profile_id": snapshot.SelectedProfileID,
		})
	})
	return mux
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
