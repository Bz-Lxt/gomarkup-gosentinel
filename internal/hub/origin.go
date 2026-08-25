package hub

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

func CheckOrigin(whitelist []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(whitelist))
	for _, o := range whitelist {
		allowed[strings.TrimRight(strings.ToLower(o), "/")] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		key := strings.ToLower(strings.TrimRight(origin, "/"))
		if _, ok := allowed[key]; ok {
			return true
		}
		reqHost := r.Host
		if strings.EqualFold(u.Host, reqHost) {
			return true
		}
		ohost, oport, _ := net.SplitHostPort(u.Host)
		rhost, _, err := net.SplitHostPort(reqHost)
		if err != nil {
			rhost = reqHost
		}
		if ohost == "" {
			ohost = u.Host
		}
		if isLoopback(ohost) && isLoopback(rhost) {
			_ = oport
			return true
		}
		return false
	}
}

func isLoopback(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
