package config

import (
	"os"
	"strings"
)

type Config struct {
	Listen    string
	DataDir   string
	LogLevel  string
	WSOrigins []string
}

func Load() Config {
	c := Config{
		Listen:   env("GOSENTINEL_LISTEN", ":8080"),
		DataDir:  env("GOSENTINEL_DATA_DIR", "./data"),
		LogLevel: env("GOSENTINEL_LOG_LEVEL", "info"),
	}
	raw := env("GOSENTINEL_WS_ORIGINS", "http://localhost:31481,http://127.0.0.1:31481")
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			c.WSOrigins = append(c.WSOrigins, t)
		}
	}
	return c
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
