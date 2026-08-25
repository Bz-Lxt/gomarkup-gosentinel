package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gosentinel/internal/hub"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
)

func TestHealthAndRuleValidation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	agg := telemetry.New()
	h := hub.New(st, agg, nil)
	s := &Server{Store: st, Hub: h, Agg: agg, Ready: func() bool { return true }}
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("health %d", res.StatusCode)
	}

	body := []byte(`{"service":"","resource":"/x","qps":0}`)
	res, err = http.Post(ts.URL+"/api/v1/rules", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnprocessableEntity && res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}

	good := []byte(`{"service":"svc","resource":"/x","qps":20,"mode":"fixed","enabled":true,"adaptive_min_qps":1,"adaptive_decrease":0.7,"adaptive_increase":5,"adaptive_latency_ms":200,"adaptive_error_rate":0.3,"adaptive_hysteresis":3,"error_rate":0.5,"min_requests":20,"open_timeout_ms":5000,"half_open_probes":3}`)
	res, err = http.Post(ts.URL+"/api/v1/rules", "application/json", bytes.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, buf.String())
	}
	var env map[string]any
	_ = json.NewDecoder(res.Body).Decode(&env)
	if env["data"] == nil {
		t.Fatal("envelope")
	}
}
