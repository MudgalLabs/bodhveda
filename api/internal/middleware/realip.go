package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ProxiedClientIP sets r.RemoteAddr to the client address recorded by the
// reverse proxy in front of the API, for the rate limiters and request logs.
//
// ⚠️ Replaces chi's middleware.RealIP, which must NOT be used here. RealIP
// trusts True-Client-IP first, then X-Real-IP, then X-Forwarded-For — and
// prod's Caddy (a bare `reverse_proxy`) passes the first two through from the
// client untouched. Any caller could therefore name its own IP per request and
// walk straight past httprate.LimitByIP on the developer and console APIs.
//
// Caddy, by contrast, does not trust an incoming X-Forwarded-For: it sets the
// header from the TCP peer it actually saw. So this reads ONLY X-Forwarded-For,
// and only its LAST entry — the hop appended by the proxy nearest the API, which
// is the one value a client cannot forge even behind a proxy that appends rather
// than replaces. With no header (local dev, no proxy) RemoteAddr is left as the
// real socket peer.
func ProxiedClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := lastForwardedFor(r.Header.Values("X-Forwarded-For")); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

func lastForwardedFor(values []string) string {
	if len(values) == 0 {
		return ""
	}

	// Multiple header lines are one list, in order.
	joined := strings.Join(values, ",")
	parts := strings.Split(joined, ",")
	ip := strings.TrimSpace(parts[len(parts)-1])

	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}
