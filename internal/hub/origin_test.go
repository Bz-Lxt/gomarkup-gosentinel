package hub

import (
	"net/http"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	fn := CheckOrigin([]string{"http://localhost:31481"})
	req := func(origin, host string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "http://"+host+"/ws", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	if !fn(req("", "localhost:8080")) {
		t.Fatal("empty origin")
	}
	if !fn(req("http://localhost:31481", "control:8080")) {
		t.Fatal("whitelist")
	}
	if !fn(req("http://localhost:8080", "localhost:8080")) {
		t.Fatal("same origin")
	}
	if !fn(req("http://127.0.0.1:9000", "localhost:8080")) {
		t.Fatal("loopback")
	}
	if fn(req("http://evil.example", "localhost:8080")) {
		t.Fatal("evil should fail")
	}
}
