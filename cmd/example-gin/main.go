package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"gosentinel/internal/config"
	"gosentinel/internal/log"
	"gosentinel/internal/node"
	ginsentinel "gosentinel/pkg/middleware/gin"
	"gosentinel/pkg/sentinel"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	cfg := config.Load()
	log.Init(cfg.LogLevel, os.Stdout)
	service := env("GOSENTINEL_SERVICE", "demo-gin")
	instance := env("HOSTNAME", "gin-1")
	guard := sentinel.New(sentinel.Options{Service: service})
	guard.RegisterFallback("/work", func(ctx context.Context, resource string, reason sentinel.Reason) error {
		return sentinel.ErrBlocked{Reason: reason, Resource: resource}
	})

	client := &node.Client{
		URL:      env("GOSENTINEL_CONTROL", "ws://127.0.0.1:31482/ws/nodes"),
		NodeID:   service + "-" + instance,
		Service:  service,
		Instance: instance,
		Guard:    guard,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go client.Run(ctx)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	protected := r.Group("/")
	protected.Use(ginsentinel.Middleware(guard, ginsentinel.Options{
		Resource: func(c *gin.Context) string { return c.FullPath() },
	}))
	protected.GET("/work", handleWork)
	protected.POST("/load", handleLoad)

	srv := &http.Server{Addr: cfg.Listen, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Logger().Info("example-gin listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Logger().Error("gin listen", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
}

func handleWork(c *gin.Context) {
	if c.Query("fail") == "1" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forced"})
		return
	}
	if ms, _ := strconv.Atoi(c.Query("delay_ms")); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func handleLoad(c *gin.Context) {
	var req struct {
		Count     int     `json:"count"`
		FailRatio float64 `json:"fail_ratio"`
		DelayMs   int     `json:"delay_ms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Count <= 0 {
		req.Count = 50
	}
	ok, fail := 0, 0
	for i := 0; i < req.Count; i++ {
		w := httptestWriter(c, req.FailRatio, i, req.DelayMs)
		if w >= 500 {
			fail++
		} else {
			ok++
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": ok, "fail": fail, "count": req.Count})
}

func httptestWriter(c *gin.Context, failRatio float64, i, delay int) int {
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	if failRatio > 0 && float64(i%100)/100 < failRatio {
		return 500
	}
	return 200
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
