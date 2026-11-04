package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxiedClientIP(t *testing.T) {
	const peer = "10.0.0.2:51234"

	cases := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{name: "no proxy header keeps the socket peer", want: peer},
		{name: "caddy-set forwarded-for is used", headers: map[string][]string{"X-Forwarded-For": {"203.0.113.7"}}, want: "203.0.113.7"},
		{
			// The bypass this replaces: chi's RealIP believed these, and Caddy
			// forwards them from the client untouched.
			name: "client-supplied True-Client-IP and X-Real-IP are ignored",
			headers: map[string][]string{
				"True-Client-IP":  {"1.1.1.1"},
				"X-Real-IP":       {"2.2.2.2"},
				"X-Forwarded-For": {"203.0.113.7"},
			},
			want: "203.0.113.7",
		},
		{
			name:    "spoofed-only headers without forwarded-for fall back to the peer",
			headers: map[string][]string{"True-Client-IP": {"1.1.1.1"}, "X-Real-IP": {"2.2.2.2"}},
			want:    peer,
		},
		{
			// A proxy that appends: the client controls everything but the last hop.
			name:    "only the last forwarded-for hop is trusted",
			headers: map[string][]string{"X-Forwarded-For": {"9.9.9.9, 8.8.8.8", "203.0.113.7"}},
			want:    "203.0.113.7",
		},
		{name: "garbage is ignored", headers: map[string][]string{"X-Forwarded-For": {"not-an-ip"}}, want: peer},
		{name: "ipv6", headers: map[string][]string{"X-Forwarded-For": {"2001:db8::1"}}, want: "2001:db8::1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			h := ProxiedClientIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = peer
			for k, vs := range c.headers {
				for _, v := range vs {
					req.Header.Add(k, v)
				}
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != c.want {
				t.Errorf("RemoteAddr = %q, want %q", got, c.want)
			}
		})
	}
}
