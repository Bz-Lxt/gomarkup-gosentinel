package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gosentinel/internal/api"
	"gosentinel/internal/config"
	"gosentinel/internal/hub"
	"gosentinel/internal/log"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
)

func main() {
	cfg := config.Load()
	log.Init(cfg.LogLevel, os.Stdout)
	logger := log.Logger()

	st, err := store.Open(filepath.Join(cfg.DataDir, "rules.json"))
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	agg := telemetry.New()
	h := hub.New(st, agg, cfg.WSOrigins)
	ready := true
	srvAPI := &api.Server{Store: st, Hub: h, Agg: agg, Ready: func() bool { return ready }}

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srvAPI.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				h.PublishDashboard(map[string]any{
					"summary":     agg.Summary(60 * time.Second),
					"metrics":     agg.Query("", "", "", 60*time.Second),
					"nodes":       h.Nodes(),
					"version":     st.Version(),
					"convergence": h.Convergence(st.Version()),
				})
			}
		}
	}()

	go func() {
		logger.Info("control plane listening", "addr", cfg.Listen)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	ready = false
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
	logger.Info("control plane stopped")
}
