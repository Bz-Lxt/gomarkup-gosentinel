package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gosentinel/internal/hub"
	"gosentinel/internal/rule"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
	"gosentinel/internal/timeutil"
)

type Server struct {
	Store *store.FileStore
	Hub   *hub.Hub
	Agg   *telemetry.Aggregator
	Ready func() bool
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/rules", s.listRules)
	mux.HandleFunc("POST /api/v1/rules", s.createRule)
	mux.HandleFunc("GET /api/v1/rules/{id}", s.getRule)
	mux.HandleFunc("PUT /api/v1/rules/{id}", s.updateRule)
	mux.HandleFunc("PATCH /api/v1/rules/{id}", s.patchRule)
	mux.HandleFunc("DELETE /api/v1/rules/{id}", s.deleteRule)
	mux.HandleFunc("POST /api/v1/rules/{id}/reset", s.resetRule)
	mux.HandleFunc("GET /api/v1/nodes", s.nodes)
	mux.HandleFunc("GET /api/v1/metrics", s.metrics)
	mux.HandleFunc("GET /api/v1/overview", s.overview)
	mux.HandleFunc("GET /api/v1/schema", s.schema)
	mux.Handle("/ws/nodes", http.HandlerFunc(s.Hub.ServeNodes))
	mux.Handle("/ws/dashboard", http.HandlerFunc(s.Hub.ServeDashboard))
	return withRecover(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	WriteData(w, http.StatusOK, map[string]any{"status": "ok", "time": timeutil.Format(timeutil.Now())})
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	if s.Ready != nil && !s.Ready() {
		WriteError(w, http.StatusServiceUnavailable, "not_ready", "not ready", nil)
		return
	}
	WriteData(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) listRules(w http.ResponseWriter, _ *http.Request) {
	WriteData(w, http.StatusOK, map[string]any{
		"version": s.Store.Version(),
		"rules":   s.Store.List(),
	})
}

func (s *Server) getRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, ok := s.Store.Get(id)
	if !ok {
		WriteError(w, http.StatusNotFound, "not_found", "rule not found", nil)
		return
	}
	WriteData(w, http.StatusOK, item)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var in rule.Snapshot
	if err := DecodeJSON(r, &in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	saved, err := s.Store.Upsert(in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Hub.BroadcastRules()
	w.Header().Set("Location", "/api/v1/rules/"+saved.ID)
	WriteData(w, http.StatusCreated, saved)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.Store.Get(id); !ok {
		WriteError(w, http.StatusNotFound, "not_found", "rule not found", nil)
		return
	}
	var in rule.Snapshot
	if err := DecodeJSON(r, &in); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	in.ID = id
	saved, err := s.Store.Upsert(in)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Hub.BroadcastRules()
	WriteData(w, http.StatusOK, map[string]any{
		"rule":        saved,
		"convergence": s.Hub.Convergence(saved.Version),
	})
}

func (s *Server) patchRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, ok := s.Store.Get(id)
	if !ok {
		WriteError(w, http.StatusNotFound, "not_found", "rule not found", nil)
		return
	}
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	if v, ok := patch["enabled"].(bool); ok {
		cur.Enabled = v
	}
	saved, err := s.Store.Upsert(cur)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.Hub.BroadcastRules()
	WriteData(w, http.StatusOK, saved)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Delete(r.PathValue("id")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			WriteError(w, http.StatusNotFound, "not_found", "rule not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), nil)
		return
	}
	s.Hub.BroadcastRules()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resetRule(w http.ResponseWriter, r *http.Request) {
	item, ok := s.Store.Get(r.PathValue("id"))
	if !ok {
		WriteError(w, http.StatusNotFound, "not_found", "rule not found", nil)
		return
	}
	s.Hub.BroadcastReset(item.Service, item.Resource, item.Method)
	WriteData(w, http.StatusOK, map[string]any{"reset": true, "rule": item.ID})
}

func (s *Server) nodes(w http.ResponseWriter, _ *http.Request) {
	WriteData(w, http.StatusOK, s.Hub.Nodes())
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	win := 60 * time.Second
	if raw := q.Get("range"); raw != "" {
		if n, err := strconv.Atoi(strings.TrimSuffix(raw, "s")); err == nil {
			win = time.Duration(n) * time.Second
		}
	}
	WriteData(w, http.StatusOK, s.Agg.Query(q.Get("service"), q.Get("resource"), q.Get("instance"), win))
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	win := 60 * time.Second
	WriteData(w, http.StatusOK, map[string]any{
		"summary":     s.Agg.Summary(win),
		"nodes":       s.Hub.Nodes(),
		"version":     s.Store.Version(),
		"convergence": s.Hub.Convergence(s.Store.Version()),
		"dropped":     s.Agg.Dropped(),
		"time":        timeutil.Format(timeutil.Now()),
	})
}

func (s *Server) schema(w http.ResponseWriter, _ *http.Request) {
	WriteData(w, http.StatusOK, FieldSchema())
}

func writeStoreErr(w http.ResponseWriter, err error) {
	var ve *rule.ValidationError
	if errors.As(err, &ve) {
		WriteError(w, http.StatusUnprocessableEntity, "validation_error", "request validation failed", ve.Details)
		return
	}
	WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), nil)
}

func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				WriteError(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func FieldSchema() []map[string]any {
	return []map[string]any{
		{"field": "service", "type": "string", "required": true, "max": 64},
		{"field": "resource", "type": "string", "required": true, "max": 128},
		{"field": "method", "type": "string", "required": false, "max": 32},
		{"field": "qps", "type": "number", "min": 1, "max": 1000000},
		{"field": "mode", "type": "enum", "values": []string{"fixed", "adaptive"}},
		{"field": "adaptive_min_qps", "type": "number", "min": 1, "max": 1000000},
		{"field": "adaptive_decrease", "type": "number", "min": 0.01, "max": 0.99},
		{"field": "adaptive_increase", "type": "number", "min": 1, "max": 10000},
		{"field": "adaptive_latency_ms", "type": "number", "min": 1, "max": 60000},
		{"field": "adaptive_error_rate", "type": "number", "min": 0.01, "max": 0.99},
		{"field": "adaptive_hysteresis", "type": "number", "min": 1, "max": 60},
		{"field": "error_rate", "type": "number", "min": 0.01, "max": 0.99},
		{"field": "min_requests", "type": "number", "min": 1, "max": 100000},
		{"field": "open_timeout_ms", "type": "number", "min": 100, "max": 600000},
		{"field": "half_open_probes", "type": "number", "min": 1, "max": 32},
		{"field": "enabled", "type": "bool"},
	}
}
