package profilediscovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesEndpoint(t *testing.T) {
	handler := NewHandler(func() Snapshot {
		return Snapshot{
			Profiles:          []Profile{{ID: "local:one", Name: "Arena", Host: "127.0.0.1", Port: 8765, Token: "secret", Running: true}},
			SelectedProfileID: "local:one",
		}
	})
	req := httptest.NewRequest(http.MethodGet, Path, nil)
	req.RemoteAddr = "127.0.0.1:50000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body struct {
		Status            string    `json:"status"`
		Profiles          []Profile `json:"profiles"`
		SelectedProfileID string    `json:"selected_profile_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.SelectedProfileID != "local:one" || len(body.Profiles) != 1 {
		t.Fatalf("unexpected response: %#v", body)
	}
	if body.Profiles[0].Token != "secret" || !body.Profiles[0].Running {
		t.Fatalf("unexpected profile: %#v", body.Profiles[0])
	}
}

func TestProfilesEndpointRejectsNonLoopback(t *testing.T) {
	handler := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, Path, nil)
	req.RemoteAddr = "192.0.2.10:50000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
