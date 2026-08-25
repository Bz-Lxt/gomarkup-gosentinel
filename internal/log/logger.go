package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu sync.RWMutex
	L  *slog.Logger
)

func init() {
	L = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func Init(level string, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	mu.Lock()
	L = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lv}))
	mu.Unlock()
}

func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if L == nil {
		return slog.Default()
	}
	return L
}
