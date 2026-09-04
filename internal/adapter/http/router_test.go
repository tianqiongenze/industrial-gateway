package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/infra/memorycache"
	"industrial-gateway/internal/infra/memoryrepo"
	"industrial-gateway/internal/usecase"
)

func newEngine() (*usecase.IngestService, *memoryrepo.Repository, *memorycache.Cache) {
	repo := memoryrepo.New()
	cache := memorycache.New()
	uc := usecase.NewIngestService(&domain.Pipeline{Steps: []*domain.Transformer{
		domain.NewTransformer(map[string]string{"c": "Celsius"}, nil)}}, repo, cache)
	return uc, repo, cache
}

func TestHealthEndpoint(t *testing.T) {
	uc, repo, cache := newEngine()
	engine := Register(uc, repo, cache)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

func TestIngestEndpoint(t *testing.T) {
	uc, repo, cache := newEngine()
	engine := Register(uc, repo, cache)
	payload := []byte(`{"device_id":"pump-1","metric":"temperature","value":96,"unit":"C","source":"rest"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("status = %d, want 201 (body=%s)", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "CRITICAL" {
		t.Errorf("status = %v, want CRITICAL", resp["status"])
	}
	if c, _ := repo.Count("pump-1"); c != 1 {
		t.Errorf("repo count = %d, want 1", c)
	}
}

func TestIngestValidation(t *testing.T) {
	uc, repo, cache := newEngine()
	engine := Register(uc, repo, cache)
	// missing device_id (binding required) -> 400
	payload := []byte(`{"metric":"temperature","value":10}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLatestEndpoint(t *testing.T) {
	uc, repo, cache := newEngine()
	engine := Register(uc, repo, cache)
	_ = repo.Save(domain.Reading{DeviceID: "d", Metric: "temperature", Value: 1, Unit: "C", Timestamp: timeNow(), Source: "rest", Quality: "good"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/readings/d", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	uc, repo, cache := newEngine()
	engine := Register(uc, repo, cache)
	cache.Set("status:pump-1", "WARNING", 0)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status/pump-1", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "WARNING" {
		t.Errorf("status = %v, want WARNING", body["status"])
	}
}
