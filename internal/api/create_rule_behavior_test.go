package api_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gosentinel/internal/api"
	"gosentinel/internal/hub"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
)

func TestCreateValidRuleReturnsCreated(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	agg := telemetry.New()
	h := hub.New(st, agg, nil)
	handler := (&api.Server{Store: st, Hub: h, Agg: agg}).Routes()

	body := `{"service":"checkout","resource":"/payments","method":"POST","enabled":true,"mode":"fixed","qps":50,"adaptive_min_qps":10,"adaptive_decrease":0.7,"adaptive_increase":5,"adaptive_latency_ms":200,"adaptive_error_rate":0.3,"adaptive_hysteresis":3,"error_rate":0.5,"min_requests":20,"open_timeout_ms":5000,"half_open_probes":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/rules status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location == "" {
		t.Fatal("POST /api/v1/rules did not return the created rule location")
	}
}
